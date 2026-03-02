package govee

import "encoding/json"

// NewAPIRequest creates a new API request wrapped with the common
// API fields.
func newAPIRequest(cmd string, data any) (*wrapper, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	msg := wrapper{}
	msg.MSG.CMD = cmd
	msg.MSG.Data = jsonData

	return &msg, nil
}

// wrapper is a generic wrapper for all API requests and responses.
type wrapper struct {
	MSG struct {
		CMD  string          `json:"cmd"`
		Data json.RawMessage `json:"data"`
	} `json:"msg"`
}

// ScanRequest represents a request to scan for devices sent to
// the multicast address.
type scanRequest struct {
	AccountTopic string `json:"account_topic"`
}

// ScanResponse represents a response to a scan request from a
// govee device.
type scanResponse struct {
	IP              string  `json:"ip"`
	DeviceID        string  `json:"device"`
	SKU             string  `json:"sku"`
	BleVersionHard  Version `json:"bleVersionHard"`
	BleVersionSoft  Version `json:"bleVersionSoft"`
	WifiVersionHard Version `json:"wifiVersionHard"`
	WifiVersionSoft Version `json:"wifiVersionSoft"`
}

// OnOffRequest represents a request to turn a device on or off.
// A value of '0' means Off
// A value of '1' means On
type onOffRequest struct {
	Value State `json:"value"`
}

// BrightnessRequest represents a request to set the brightness of a device.
// Value is a percentage between 0 and 100.
type brightnessRequest struct {
	Value Brightness `json:"value"`
}

// ColorKelvinRequest represents a request to set the color temperature of a device.
// When the value of the color temperature is not “0”, the device will convert the
// color temperature value into the color value of red, green and blue. When the value
// of the color temperature is “0”, the device will only resolve the value of “r”, “g”
// and “b” in the color field.
type colorRequest struct {
	Color  Color       `json:"color"`
	Kelvin ColorKelvin `json:"colorTemInKelvin"`
}

// DevStatusRequest represents a request to get the status of a device.
// This API call doesn't take any parameters, so its a placeholder
// in case it does in the future.
type devStatusRequest struct {
}

// DevStatusResponse represents a response to a device status request.
type devStatusResponse struct {
	OnOff       State       `json:"onOff"`
	Brightness  Brightness  `json:"brightness"`
	Color       Color       `json:"color"`
	ColorKelvin ColorKelvin `json:"colorTemInKelvin"`
}
