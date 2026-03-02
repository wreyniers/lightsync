package govee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// Controller manages Govee devices and communication over the network.
type Controller struct {
	logger  *slog.Logger
	devices []*Device
	ctx     context.Context
	cancel  context.CancelFunc
	command chan Message
	wg      sync.WaitGroup
}

// NewController creates a new Controller with the provided logger.
func NewController(logger *slog.Logger) *Controller {
	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		devices: []*Device{},
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		command: make(chan Message),
	}
}

// Start initializes the controller, begins listening for device messages, and starts periodic scanning for devices (every 60 seconds). Returns an error if the network cannot be initialized.
func (c *Controller) Start() error {
	c.logger.Info("Starting Govee Controller")
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:4002")
	if err != nil {
		c.logger.Error("Failed to resolve UDP address", "error", err)
		return err
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		c.logger.Error("Failed to listen on multicast UDP", "error", err)
		return err
	}
	// Don't defer conn.Close() here, close in Shutdown

	err = conn.SetReadBuffer(8192)
	if err != nil {
		c.logger.Error("Failed to set UDP read buffer", "error", err)
		return err
	}

	// Main UDP listener goroutine
	c.logger.Debug("WG Add: UDP listener goroutine")
	c.wg.Add(1)
	go func() {
		c.logger.Debug("UDP listener goroutine started")
		defer func() {
			c.logger.Debug("UDP listener goroutine exiting, calling WG Done")
			c.wg.Done()
		}()
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				// Set a short read deadline so we can check ctx.Done() regularly
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				buffer := make([]byte, 8192)
				n, src, err := conn.ReadFromUDP(buffer)
				if err != nil {
					// If timeout, just continue to check ctx.Done()
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						continue
					}
					// Expected during shutdown: conn.Close() while ReadFromUDP is blocked
					if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
						return
					}
					c.logger.Error("Error reading from UDP", "error", err)
					continue
				}

				srcAddr := src.IP.String()
				device, err := c.DeviceByIP(srcAddr)

				// New device discovered, register it and start its handler.
				if err != nil {
					c.logger.Debug("Discovered new device", "ip", srcAddr)

					deviceLogger := c.logger.With("device_ip", srcAddr)
					newDevice := Device{
						ip:           srcAddr,
						logger:       deviceLogger,
						ctx:          c.ctx,
						command:      c.command,
						response:     make(chan Message),
						statusUpdate: make(chan time.Time, 1),
					}
					go newDevice.handler()
					c.devices = append(c.devices, &newDevice)
					device = &newDevice
				}

				// Parse incoming message
				var request wrapper
				err = json.Unmarshal(buffer[:n], &request)
				if err != nil {
					c.logger.Error("Invalid API Request", "error", err)
					continue
				}

				// Handle incoming command and dispatch to device handler
				switch request.MSG.CMD {
				case "scan":
					c.logger.Debug("Received scan response", "from", srcAddr)
					msg := scanResponse{}
					err = json.Unmarshal(request.MSG.Data, &msg)
					if err != nil {
						c.logger.Error("Invalid scan response", "error", err)
						continue
					}

					device.response <- Message{IP: srcAddr, Payload: msg}

				case "devStatus":
					c.logger.Debug("Received device status", "from", srcAddr)
					msg := devStatusResponse{}
					err = json.Unmarshal(request.MSG.Data, &msg)
					if err != nil {
						c.logger.Error("Invalid device status response", "error", err)
						continue
					}

					device.response <- Message{IP: srcAddr, Payload: msg}

				default:
					c.logger.Warn("Unknown command received", "cmd", request.MSG.CMD)
				}
			}
		}
	}()

	c.logger.Debug("WG Add: command sender goroutine")
	c.wg.Add(1)
	go func() {
		c.logger.Debug("command sender goroutine started")
		defer func() {
			c.logger.Debug("command sender goroutine exiting, calling WG Done")
			c.wg.Done()
		}()
		for cmd := range c.command {
			data, err := json.Marshal(cmd.Payload)
			if err != nil {
				c.logger.Error("Failed to marshal command", "error", err)
				continue
			}

			var target string
			if cmd.IP == "239.255.255.250" {
				target = fmt.Sprintf("%s:4001", cmd.IP)
			} else {
				target = fmt.Sprintf("%s:4003", cmd.IP)
			}

			addr, err := net.ResolveUDPAddr("udp4", target)
			if err != nil {
				c.logger.Error("Failed to resolve device address", "error", err)
				continue
			}

			deviceConn, err := net.DialUDP("udp4", nil, addr)
			if err != nil {
				c.logger.Error("Failed to dial device address", "error", err)
				continue
			}

			_, err = deviceConn.Write(data)
			if err != nil {
				c.logger.Error("Failed to send command", "error", err)
				deviceConn.Close()
				continue
			}

			deviceConn.Close()
		}
	}()

	c.logger.Debug("WG Add: periodic scan goroutine")
	c.wg.Add(1)
	go func() {
		c.logger.Debug("periodic scan goroutine started")
		defer func() {
			c.logger.Debug("periodic scan goroutine exiting, calling WG Done")
			c.wg.Done()
		}()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		scan, err := newAPIRequest("scan", scanRequest{AccountTopic: "reserve"})
		if err != nil {
			c.logger.Error("Failed to create scan request", "error", err)
			return
		}
		msg := Message{"239.255.255.250", scan}

		// send immediate scan on startup
		c.command <- msg

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.logger.Debug("Sending periodic scan request")
				c.command <- msg
			}
		}
	}()

	<-c.ctx.Done()
	// Wait for all goroutines to finish
	c.logger.Debug("WG Wait: waiting for all goroutines to finish")
	conn.Close()
	close(c.command)
	c.wg.Wait()
	c.logger.Debug("WG Wait: all goroutines finished")
	return nil
}

// Shutdown gracefully shuts down the controller and all goroutines. Blocks until all background tasks have exited.
func (c *Controller) Shutdown() error {
	c.logger.Info("Shutting down Govee Controller")
	c.cancel()
	c.logger.Debug("Shutdown: waiting for WaitGroup")
	// c.command will be closed by Start after context is canceled
	c.wg.Wait()
	c.logger.Debug("Shutdown: WaitGroup finished")
	return nil
}

// Devices returns a slice of all managed devices.
func (c *Controller) Devices() []*Device {
	return c.devices
}

// DeviceByIP returns a pointer to a device by its IP address, or an error if not found.
func (c *Controller) DeviceByIP(ip string) (*Device, error) {
	for _, device := range c.devices {
		if device.ip == ip {
			return device, nil
		}
	}
	return nil, ErrNoDeviceFound
}

// DeviceByID returns a pointer to a device by its DeviceID, or an error if not found.
func (c *Controller) DeviceByID(id string) (*Device, error) {
	for _, device := range c.devices {
		if device.deviceID == id {
			return device, nil
		}
	}
	return nil, ErrNoDeviceFound
}
