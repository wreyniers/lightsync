# Go-Vee

Go-Vee is a Go library for controlling Govee smart devices over the local network. It provides a simple API to discover devices, send commands, and receive status updates using UDP multicast and unicast communication.

## Features
- Device discovery via multicast
- Turn devices on/off
- Toggle device state
- Set brightness
- Set color (RGB)
- Set color temperature (Kelvin)
- Device status and response handling

## Installation
Add Go-Vee to your project:

```sh
go get github.com/swrm-io/go-vee
```

## Usage

### 1. Create a Controller
```go
import (
    "log/slog"
    "github.com/swrm-io/go-vee"
)

logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
controller := govee.NewController(logger)
go func() {
    err := controller.Start()
    if err != nil {
        logger.Error("Failed to start controller", "error", err)
    }
}

time.Sleep(1 * time.Minute)
controller.Shutdown()
```

### 2. Discover Devices
Devices are discovered automatically. You can access them via:
```go
devices := controller.Devices()
```
or by IP
```go
mydevice := controller.DeviceByIP("192.168.0.130")
```

### 3. Send Commands
```go
device := controller.DeviceByIP("192.168.0.130")
device.TurnOn()
device.SetBrightness(80)
device.SetColor(govee.Color{R: 255, G: 0, B: 0}) // Red
```

## Contributing
Pull requests and issues are welcome!

## License
Apache License 2.0
See the LICENSE file for details.
