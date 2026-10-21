package govee

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Device represents a Govee device with its properties and current state.
// It manages device state, communication, and provides control methods.
type Device struct {
	seen time.Time

	ip              string
	deviceID        string
	sku             string
	bleVersionHard  Version
	bleVersionSoft  Version
	wifiVersionHard Version
	wifiVersionSoft Version

	state       State
	brightness  Brightness
	color       Color
	colorKelvin ColorKelvin

	logger       *slog.Logger
	ctx          context.Context
	command      chan Message
	response     chan Message
	statusUpdate chan time.Time
}

// handler listens for device responses and updates device state. Exits when ctx is canceled.
func (d *Device) handler() {
	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("Device handler context canceled, exiting")
			return
		case resp, ok := <-d.response:
			if !ok {
				d.logger.Info("Device response channel closed, exiting handler")
				return
			}
			switch payload := resp.Payload.(type) {
			case scanResponse:
				d.logger.Info("Discovered device", "ip", payload.IP, "deviceID", payload.DeviceID, "sku", payload.SKU)
				d.ip = payload.IP
				d.deviceID = payload.DeviceID
				d.sku = payload.SKU
				d.bleVersionHard = payload.BleVersionHard
				d.bleVersionSoft = payload.BleVersionSoft
				d.wifiVersionHard = payload.WifiVersionHard
				d.wifiVersionSoft = payload.WifiVersionSoft
				d.seen = time.Now()

			case devStatusResponse:
				d.logger.Info("Device status update", "onOff", payload.OnOff, "brightness", payload.Brightness, "color", payload.Color, "colorKelvin", payload.ColorKelvin)
				d.state = payload.OnOff
				d.brightness = payload.Brightness
				d.color = payload.Color
				d.colorKelvin = payload.ColorKelvin
				d.seen = time.Now()
				select {
				case d.statusUpdate <- time.Now():
				default:
				}
			default:
				d.logger.Warn("Unknown command type", "type", fmt.Sprintf("%T", resp))
			}
		}
	}
}

// String returns a string representation of the device.
func (d *Device) String() string {
	var sku = "unknown"
	if d.sku != "" {
		sku = d.sku
	}

	var deviceID = "unknown"
	if d.deviceID != "" {
		deviceID = d.deviceID
	}
	return fmt.Sprintf("%s: %s (%s)", sku, d.ip, deviceID)
}

// Active returns true if the device has been seen in the last 5 minutes.
func (d *Device) Active() bool {
	return time.Since(d.seen) < 5*time.Minute
}

// IP returns the device's IP address.
func (d *Device) IP() string { return d.ip }

// DeviceID returns the device's unique identifier.
func (d *Device) DeviceID() string { return d.deviceID }

// SKU returns the device's SKU.
func (d *Device) SKU() string { return d.sku }

// BleVersionHard returns the BLE hardware version.
func (d *Device) BleVersionHard() Version { return d.bleVersionHard }

// BleVersionSoft returns the BLE software version.
func (d *Device) BleVersionSoft() Version { return d.bleVersionSoft }

// WifiVersionHard returns the WiFi hardware version.
func (d *Device) WifiVersionHard() Version { return d.wifiVersionHard }

// WifiVersionSoft returns the WiFi software version.
func (d *Device) WifiVersionSoft() Version { return d.wifiVersionSoft }

// State returns the current on/off state of the device.
func (d *Device) State() State { return d.state }

// Brightness returns the current brightness of the device.
func (d *Device) Brightness() Brightness { return d.brightness }

// Color returns the current color of the device.
func (d *Device) Color() Color { return d.color }

// ColorKelvin returns the current color temperature of the device.
func (d *Device) ColorKelvin() ColorKelvin { return d.colorKelvin }

// TurnOn turns the device on. Returns an error if the command cannot be sent.
func (d *Device) TurnOn() error {
	d.logger.Debug("Sending Turn On command")
	cmd := onOffRequest{Value: 1}
	wrapper, err := newAPIRequest("turn", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
		return nil
	default:
		return fmt.Errorf("failed to send TurnOn command: channel blocked or closed")
	}
}

// TurnOff turns the device off. Returns an error if the command cannot be sent.
func (d *Device) TurnOff() error {
	d.logger.Debug("Sending Turn Off command")
	cmd := onOffRequest{Value: 0}
	wrapper, err := newAPIRequest("turn", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
		return nil
	default:
		return fmt.Errorf("failed to send TurnOff command: channel blocked or closed")
	}
}

// Toggle toggles the device state. Returns an error if the command cannot be sent.
func (d *Device) Toggle() error {
	d.logger.Debug("Toggling device state")
	if d.state == 1 {
		return d.TurnOff()
	}
	return d.TurnOn()
}

// SetBrightness sets the brightness of the device. Returns an error if the command cannot be sent.
func (d *Device) SetBrightness(brightness Brightness) error {
	d.logger.Debug("Setting brightness", "brightness", brightness)
	cmd := brightnessRequest{Value: brightness}
	wrapper, err := newAPIRequest("brightness", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
		return nil
	default:
		return fmt.Errorf("failed to send SetBrightness command: channel blocked or closed")
	}
}

// SetColor sets the color of the device. Returns an error if the command cannot be sent.
func (d *Device) SetColor(color Color) error {
	d.logger.Debug("Setting color", "color", color)
	cmd := colorRequest{Color: color, Kelvin: 0}
	wrapper, err := newAPIRequest("colorwc", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
		return nil
	default:
		return fmt.Errorf("failed to send SetColor command: channel blocked or closed")
	}
}

// SetColorKelvin sets the color temperature of the device. Returns an error if the command cannot be sent.
func (d *Device) SetColorKelvin(colorKelvin ColorKelvin) error {
	d.logger.Debug("Setting color temperature", "colorKelvin", colorKelvin)
	cmd := colorRequest{Color: Color{}, Kelvin: colorKelvin}
	wrapper, err := newAPIRequest("colorwc", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
		return nil
	default:
		return fmt.Errorf("failed to send SetColorKelvin command: channel blocked or closed")
	}
}

// RequestStatus requests the current status of the device and blocks until a response is received or times out after 5 seconds. Returns an error if the command cannot be sent or if the response times out.
func (d *Device) RequestStatus() error {
	d.logger.Debug("Requesting device status")
	cmd := devStatusRequest{}
	wrapper, err := newAPIRequest("devStatus", cmd)
	if err != nil {
		return err
	}
	select {
	case d.command <- Message{IP: d.ip, Payload: wrapper}:
	case <-d.ctx.Done():
		return fmt.Errorf("context canceled while sending RequestStatus command")
	default:
		return fmt.Errorf("failed to send RequestStatus command: channel blocked or closed")
	}
	// Wait for status update
	select {
	case <-d.statusUpdate:
		d.logger.Debug("Received status response")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for device status response")
	case <-d.ctx.Done():
		return fmt.Errorf("context canceled while waiting for status response")
	}
}
