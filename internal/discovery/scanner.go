package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"

	"lightsync/internal/lights"
)

type Scanner struct {
	lightManager *lights.Manager
	elgatoCtrl   *lights.ElgatoController
}

func NewScanner(lm *lights.Manager, elgato *lights.ElgatoController) *Scanner {
	return &Scanner{
		lightManager: lm,
		elgatoCtrl:   elgato,
	}
}

type DiscoveryResult struct {
	Devices []lights.Device `json:"devices"`
	Errors  []string        `json:"errors,omitempty"`
}

type ScanProgress struct {
	Phase   string          `json:"phase"`
	Message string          `json:"message"`
	Devices []lights.Device `json:"devices,omitempty"`
}

func (s *Scanner) ScanAll(ctx context.Context, onProgress func(ScanProgress)) DiscoveryResult {
	result := DiscoveryResult{}
	progress := func(phase, message string, devices []lights.Device) {
		log.Printf("[discovery] %s", message)
		if onProgress != nil {
			onProgress(ScanProgress{Phase: phase, Message: message, Devices: devices})
		}
	}

	progress("elgato", "Searching for Elgato lights...", nil)
	mdnsFound := s.discoverElgatoViaMDNS(ctx)
	log.Printf("[discovery] mDNS found %d Elgato device(s)", mdnsFound)

	if mdnsFound == 0 {
		progress("elgato", "Scanning subnet for Elgato lights...", nil)
		s.discoverElgatoViaProbe(ctx)
	}

	progress("hue", "Searching for Hue bridges...", nil)
	hueBridges := s.DiscoverHueBridges(ctx)
	if len(hueBridges) > 0 {
		progress("hue", fmt.Sprintf("Found %d Hue bridge(s), querying lights...", len(hueBridges)), nil)
	}

	progress("lights", "Querying all bridges and devices for lights...", nil)
	var totalFound int
	devices, err := s.lightManager.DiscoverAllWithProgress(ctx, func(newDevices []lights.Device) {
		totalFound += len(newDevices)
		progress("lights", fmt.Sprintf("Found %d light(s)...", totalFound), newDevices)
	})
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.Devices = devices

	progress("done", fmt.Sprintf("Scan complete — found %d light(s)", len(devices)), nil)
	return result
}

func (s *Scanner) discoverElgatoViaMDNS(ctx context.Context) int {
	entries := make(chan *mdns.ServiceEntry, 10)
	found := 0

	go func() {
		params := &mdns.QueryParam{
			Service:             "_elg._tcp",
			Domain:              "local",
			Timeout:             3 * time.Second,
			Entries:             entries,
			DisableIPv6:         true,
			WantUnicastResponse: true,
		}
		if err := mdns.Query(params); err != nil {
			log.Printf("[discovery] mDNS Elgato query error: %v", err)
		}
		close(entries)
	}()

	for entry := range entries {
		if ctx.Err() != nil {
			return found
		}
		log.Printf("[discovery] mDNS entry: Name=%s AddrV4=%v Port=%d", entry.Name, entry.AddrV4, entry.Port)
		addr := entry.AddrV4.String()
		if addr == "" || addr == "<nil>" {
			continue
		}
		s.elgatoCtrl.AddDevice(addr)
		found++
	}
	return found
}

func (s *Scanner) discoverElgatoViaProbe(ctx context.Context) int {
	subnets := getLocalSubnets()
	if len(subnets) == 0 {
		log.Println("[discovery] Could not determine local subnets for probe scan")
		return 0
	}

	found := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // limit concurrency

	for _, subnet := range subnets {
		log.Printf("[discovery] Probing subnet %s for Elgato lights on port 9123", subnet)
		ips := expandSubnet(subnet)
		for _, ip := range ips {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(addr string) {
				defer wg.Done()
				defer func() { <-sem }()

				if isElgatoKeyLight(ctx, addr) {
					log.Printf("[discovery] Found Elgato light at %s via probe", addr)
					s.elgatoCtrl.AddDevice(addr)
					mu.Lock()
					found++
					mu.Unlock()
				}
			}(ip)
		}
	}
	wg.Wait()
	return found
}

func isElgatoKeyLight(ctx context.Context, ip string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	url := fmt.Sprintf("http://%s:9123/elgato/accessory-info", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func getLocalSubnets() []string {
	var subnets []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			ones, bits := ipNet.Mask.Size()
			if ones == 0 || bits == 0 || ones > 24 {
				continue
			}
			subnet := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
			subnets = append(subnets, subnet)
		}
	}
	return subnets
}

func expandSubnet(prefix string) []string {
	ips := make([]string, 0, 254)
	for i := 1; i <= 254; i++ {
		ips = append(ips, fmt.Sprintf("%s.%d", prefix, i))
	}
	return ips
}

type DiscoveredHueBridge struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

func (s *Scanner) DiscoverHueBridges(ctx context.Context) []DiscoveredHueBridge {
	var mu sync.Mutex
	seen := make(map[string]bool)
	var bridges []DiscoveredHueBridge

	addBridge := func(ip, name string) {
		mu.Lock()
		defer mu.Unlock()
		if seen[ip] {
			return
		}
		seen[ip] = true
		bridges = append(bridges, DiscoveredHueBridge{IP: ip, Name: name})
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		discoverHueViaSsdp(ctx, addBridge)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		discoverHueViaCloud(ctx, addBridge)
	}()

	wg.Wait()

	mu.Lock()
	found := len(bridges)
	mu.Unlock()

	if found == 0 {
		log.Println("[discovery] SSDP and cloud found nothing, falling back to subnet probe on port 443")
		discoverHueViaProbe(ctx, addBridge)
	}

	return bridges
}

func discoverHueViaSsdp(ctx context.Context, addBridge func(ip, name string)) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		log.Printf("[discovery] SSDP: failed to open UDP socket: %v", err)
		return
	}
	defer conn.Close()

	ssdpAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		log.Printf("[discovery] SSDP: failed to resolve multicast address: %v", err)
		return
	}

	searchTargets := []string{
		"ssdp:all",
		"urn:schemas-upnp-org:device:Basic:1",
		"upnp:rootdevice",
	}

	for _, st := range searchTargets {
		msg := "M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			"ST: " + st + "\r\n" +
			"MX: 3\r\n" +
			"\r\n"
		if _, err := conn.WriteTo([]byte(msg), ssdpAddr); err != nil {
			log.Printf("[discovery] SSDP: failed to send M-SEARCH for %s: %v", st, err)
		} else {
			log.Printf("[discovery] SSDP: sent M-SEARCH for %s", st)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline.Add(-500 * time.Millisecond)
	}

	buf := make([]byte, 4096)
	seen := make(map[string]bool)
	responseCount := 0

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("[discovery] SSDP: read error: %v", err)
			continue
		}

		responseCount++
		udpAddr, ok := addr.(*net.UDPAddr)
		if !ok {
			continue
		}
		ip := udpAddr.IP.String()

		response := string(buf[:n])
		responseUpper := strings.ToUpper(response)
		isHue := strings.Contains(responseUpper, "HUE") ||
			strings.Contains(responseUpper, "PHILIPS") ||
			strings.Contains(responseUpper, "IPBRIDGE")

		if !isHue {
			continue
		}

		if seen[ip] {
			continue
		}
		seen[ip] = true

		log.Printf("[discovery] SSDP found Hue bridge at %s", ip)
		addBridge(ip, "Hue Bridge")
	}
	log.Printf("[discovery] SSDP: received %d total responses, found %d Hue bridge(s)", responseCount, len(seen))
}

func discoverHueViaCloud(ctx context.Context, addBridge func(ip, name string)) {
	nupnpURLs := []string{
		"https://discovery.meethue.com/",
		"https://www.meethue.com/api/nupnp",
		"http://www.meethue.com/api/nupnp",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, url := range nupnpURLs {
		if ctx.Err() != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[discovery] Hue N-UPnP %s error: %v", url, err)
			continue
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			log.Printf("[discovery] Hue N-UPnP %s rate limited (429), trying next", url)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			log.Printf("[discovery] Hue N-UPnP %s returned %d", url, resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var results []struct {
			ID                string `json:"id"`
			InternalIPAddress string `json:"internalipaddress"`
			Port              int    `json:"port"`
		}
		if err := json.Unmarshal(body, &results); err != nil {
			log.Printf("[discovery] Hue N-UPnP %s parse error: %v", url, err)
			continue
		}

		found := false
		for _, r := range results {
			if r.InternalIPAddress != "" {
				name := "Hue Bridge"
				if len(r.ID) >= 6 {
					name = "Hue Bridge (" + r.ID[len(r.ID)-6:] + ")"
				}
				log.Printf("[discovery] N-UPnP found Hue bridge at %s (id: %s)", r.InternalIPAddress, r.ID)
				addBridge(r.InternalIPAddress, name)
				found = true
			}
		}
		if found {
			return
		}
	}
}

func discoverHueViaProbe(ctx context.Context, addBridge func(ip, name string)) {
	subnets := getLocalSubnets()
	if len(subnets) == 0 {
		log.Println("[discovery] Could not determine local subnets for Hue probe")
		return
	}

	client := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 80)

	for _, subnet := range subnets {
		log.Printf("[discovery] Probing subnet %s for Hue bridges on port 443", subnet)
		ips := expandSubnet(subnet)
		for _, ip := range ips {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(addr string) {
				defer wg.Done()
				defer func() { <-sem }()
				if isHueBridge(ctx, client, addr) {
					log.Printf("[discovery] Probe found Hue bridge at %s", addr)
					addBridge(addr, "Hue Bridge")
				}
			}(ip)
		}
	}
	wg.Wait()
}

func isHueBridge(ctx context.Context, httpsClient *http.Client, ip string) bool {
	urls := []string{
		fmt.Sprintf("http://%s/api/config", ip),
		fmt.Sprintf("https://%s/api/0/config", ip),
	}

	for _, url := range urls {
		client := httpsClient
		if strings.HasPrefix(url, "http://") {
			client = &http.Client{Timeout: 1 * time.Second}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if err != nil {
			continue
		}

		var config struct {
			BridgeID string `json:"bridgeid"`
		}
		if err := json.Unmarshal(body, &config); err != nil {
			continue
		}
		if config.BridgeID != "" {
			return true
		}
	}
	return false
}
