package webcam

import (
	"log"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Media Foundation sensor activity monitor (IMFSensorActivityMonitor).
// Windows pushes a report whenever a process starts or stops streaming from a
// camera; we keep the latest per-device state and answer isCameraOn from it.

var (
	modmfplat        = windows.NewLazySystemDLL("mfplat.dll")
	modmfsensorgroup = windows.NewLazySystemDLL("mfsensorgroup.dll")

	procMFStartup                     = modmfplat.NewProc("MFStartup")
	procMFCreateSensorActivityMonitor = modmfsensorgroup.NewProc("MFCreateSensorActivityMonitor")
)

const (
	mfVersion     = 0x00020070 // MF_SDK_VERSION<<16 | MF_API_VERSION
	mfStartupLite = 1
	eNoInterface  = 0x80004002
)

var (
	iidIUnknown       = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidReportCallback = windows.GUID{Data1: 0xDE5072EE, Data2: 0xDBE3, Data3: 0x46DC, Data4: [8]byte{0x8A, 0x87, 0xB6, 0xF6, 0x31, 0x19, 0x47, 0x51}}
)

// vtable slots after IUnknown (0 QueryInterface, 1 AddRef, 2 Release)
const (
	monitorStart = 3

	reportsGetCount  = 3
	reportsGetReport = 4

	reportGetFriendlyName    = 3
	reportGetSymbolicLink    = 4
	reportGetProcessCount    = 5
	reportGetProcessActivity = 6
	processGetProcessID      = 3
	processGetStreamingState = 4
	iunknownRelease          = 2
)

// sensorCallback is a static IMFSensorActivitiesReportCallback. It lives for
// the whole process, so AddRef/Release don't need to track anything.
type sensorCallback struct {
	vtbl *sensorCallbackVtbl
}

type sensorCallbackVtbl struct {
	queryInterface     uintptr
	addRef             uintptr
	release            uintptr
	onActivitiesReport uintptr
}

var callbackObj = &sensorCallback{vtbl: &sensorCallbackVtbl{
	queryInterface:     syscall.NewCallback(cbQueryInterface),
	addRef:             syscall.NewCallback(cbAddRef),
	release:            syscall.NewCallback(cbRelease),
	onActivitiesReport: syscall.NewCallback(cbOnActivitiesReport),
}}

var (
	sensorOnce sync.Once
	sensorOK   bool

	sensorMu      sync.Mutex
	sensorDevices = map[string][]uint32{} // symbolic link -> streaming PIDs

	// sensorChanged is signalled (coalesced) whenever a device's streaming set changes.
	sensorChanged = make(chan struct{}, 1)
)

// cameraEvents returns a channel that fires when camera activity changes, or
// nil when push notifications are unavailable and the caller must poll.
func cameraEvents() <-chan struct{} {
	if startSensorMonitor() {
		return sensorChanged
	}
	return nil
}

// startSensorMonitor starts the monitor on first use and reports whether it
// is running.
func startSensorMonitor() bool {
	sensorOnce.Do(func() {
		if err := procMFCreateSensorActivityMonitor.Find(); err != nil {
			log.Printf("[webcam] Sensor activity monitor unavailable, using registry: %v", err)
			return
		}

		ready := make(chan bool)
		go func() {
			// Keep this thread in the MTA for the life of the process so the
			// monitor keeps delivering callbacks.
			runtime.LockOSThread()

			if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil {
				log.Printf("[webcam] CoInitializeEx failed: %v", err)
				ready <- false
				return
			}
			if hr, _, _ := procMFStartup.Call(mfVersion, mfStartupLite); failed(hr) {
				log.Printf("[webcam] MFStartup failed: 0x%08X", hr)
				ready <- false
				return
			}

			var monitor uintptr
			hr, _, _ := procMFCreateSensorActivityMonitor.Call(
				uintptr(unsafe.Pointer(callbackObj)),
				uintptr(unsafe.Pointer(&monitor)),
			)
			if failed(hr) || monitor == 0 {
				log.Printf("[webcam] MFCreateSensorActivityMonitor failed: 0x%08X", hr)
				ready <- false
				return
			}
			if hr, _, _ := syscall.SyscallN(method(monitor, monitorStart), monitor); failed(hr) {
				log.Printf("[webcam] Sensor activity monitor Start failed: 0x%08X", hr)
				ready <- false
				return
			}

			log.Println("[webcam] Sensor activity monitor started")
			ready <- true
			select {}
		}()
		sensorOK = <-ready
	})
	return sensorOK
}

func sensorCameraOn() bool {
	sensorMu.Lock()
	defer sensorMu.Unlock()
	for _, pids := range sensorDevices {
		if len(pids) > 0 {
			return true
		}
	}
	return false
}

func cbQueryInterface(this, riid, ppv uintptr) uintptr {
	iid := *(**windows.GUID)(unsafe.Pointer(&riid))
	out := *(**uintptr)(unsafe.Pointer(&ppv))
	if *iid == iidIUnknown || *iid == iidReportCallback {
		*out = this
		return 0
	}
	*out = 0
	return eNoInterface
}

func cbAddRef(this uintptr) uintptr  { return 1 }
func cbRelease(this uintptr) uintptr { return 1 }

func cbOnActivitiesReport(this, reports uintptr) uintptr {
	var count uint32
	if hr, _, _ := syscall.SyscallN(method(reports, reportsGetCount), reports, uintptr(unsafe.Pointer(&count))); failed(hr) {
		return 0
	}

	for i := uint32(0); i < count; i++ {
		var report uintptr
		hr, _, _ := syscall.SyscallN(method(reports, reportsGetReport), reports, uintptr(i), uintptr(unsafe.Pointer(&report)))
		if failed(hr) || report == 0 {
			continue
		}
		link := reportString(report, reportGetSymbolicLink)
		name := reportString(report, reportGetFriendlyName)
		pids := streamingPIDs(report)
		syscall.SyscallN(method(report, iunknownRelease), report)

		sensorMu.Lock()
		prev := sensorDevices[link]
		sensorDevices[link] = pids
		sensorMu.Unlock()

		if !slices.Equal(prev, pids) {
			log.Printf("[webcam] Sensor activity: %q streaming PIDs %v", name, pids)
			select {
			case sensorChanged <- struct{}{}:
			default:
			}
		}
	}
	return 0
}

func streamingPIDs(report uintptr) []uint32 {
	var procs uint32
	if hr, _, _ := syscall.SyscallN(method(report, reportGetProcessCount), report, uintptr(unsafe.Pointer(&procs))); failed(hr) {
		return nil
	}

	var pids []uint32
	for j := uint32(0); j < procs; j++ {
		var activity uintptr
		hr, _, _ := syscall.SyscallN(method(report, reportGetProcessActivity), report, uintptr(j), uintptr(unsafe.Pointer(&activity)))
		if failed(hr) || activity == 0 {
			continue
		}
		var pid uint32
		var streaming int32
		syscall.SyscallN(method(activity, processGetProcessID), activity, uintptr(unsafe.Pointer(&pid)))
		syscall.SyscallN(method(activity, processGetStreamingState), activity, uintptr(unsafe.Pointer(&streaming)))
		syscall.SyscallN(method(activity, iunknownRelease), activity)
		if streaming != 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

func reportString(report uintptr, slot int) string {
	var buf [512]uint16
	var written uint32
	hr, _, _ := syscall.SyscallN(method(report, slot), report,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&written)))
	if failed(hr) {
		return ""
	}
	return windows.UTF16ToString(buf[:])
}

// method returns the function pointer in vtable slot idx of a COM object.
func method(obj uintptr, idx int) uintptr {
	vtbl := **(**uintptr)(unsafe.Pointer(&obj))
	return *(*uintptr)(unsafe.Add(*(*unsafe.Pointer)(unsafe.Pointer(&vtbl)), uintptr(idx)*unsafe.Sizeof(uintptr(0))))
}

func failed(hr uintptr) bool { return int32(hr) < 0 }
