package govee

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanRequest(t *testing.T) {
	data := scanRequest{AccountTopic: "reserve"}
	dataBytes, err := json.Marshal(data)
	assert.NoError(t, err, "failed to marshal JSON")
	req := wrapper{}
	req.MSG.CMD = "scan"
	req.MSG.Data = dataBytes

	jsonData := []byte(`{"msg":{"cmd":"scan","data":{"account_topic":"reserve"}}}`)
	m, err := json.Marshal(req)

	assert.NoError(t, err, "failed to marshal JSON")
	assert.Equal(t, string(jsonData), string(m), "JSON output mismatch")
}

func TestScanResult(t *testing.T) {
	jsonData := []byte(`{"msg":{"cmd":"scan","data":{"ip":"192.168.1.23","device":"1F:80:C5:32:32:36:72:4E","sku":"Hxxxx","bleVersionHard":"3.01.01","bleVersionSoft":"1.03.01","wifiVersionHard":"1.00.10","wifiVersionSoft":"1.02.03"}}}`)
	var wrapper wrapper
	err := json.Unmarshal(jsonData, &wrapper)
	assert.NoError(t, err, "failed to unmarshal wrapper")
	assert.Equal(t, "scan", wrapper.MSG.CMD, "CMD mismatch")
	var result scanResponse
	err = json.Unmarshal(wrapper.MSG.Data, &result)

	assert.NoError(t, err, "failed to unmarshal scanResponse")
	assert.Equal(t, "192.168.1.23", result.IP)
	assert.Equal(t, "1F:80:C5:32:32:36:72:4E", result.DeviceID)
	assert.Equal(t, "Hxxxx", result.SKU)
	assert.Equal(t, "3.1.1", result.BleVersionHard.String())
	assert.Equal(t, "1.3.1", result.BleVersionSoft.String())
	assert.Equal(t, "1.0.10", result.WifiVersionHard.String())
	assert.Equal(t, "1.2.3", result.WifiVersionSoft.String())
}

func TestOnOffRequest(t *testing.T) {
	data := onOffRequest{Value: 1}
	dataBytes, err := json.Marshal(data)
	assert.NoError(t, err, "failed to marshal JSON")
	req := wrapper{}
	req.MSG.CMD = "turn"
	req.MSG.Data = dataBytes

	jsonData := []byte(`{"msg":{"cmd":"turn","data":{"value":1}}}`)
	m, err := json.Marshal(req)

	assert.NoError(t, err, "failed to marshal JSON")
	assert.Equal(t, string(jsonData), string(m), "JSON output mismatch")
}

func TestDevStatusRequest(t *testing.T) {
	data := devStatusRequest{}
	dataBytes, err := json.Marshal(data)
	assert.NoError(t, err, "failed to marshal JSON")
	req := wrapper{}
	req.MSG.CMD = "devStatus"
	req.MSG.Data = dataBytes
	jsonData := []byte(`{"msg":{"cmd":"devStatus","data":{}}}`)
	m, err := json.Marshal(req)

	assert.NoError(t, err, "failed to marshal JSON")
	assert.Equal(t, string(jsonData), string(m), "JSON output mismatch")
}

func TestDevStatusResponse(t *testing.T) {
	jsonData := []byte(`{"msg":{"cmd":"devStatus","data":{"onOff":1,"brightness":100,"color":{"r":255,"g":0,"b":0},"colorTemInKelvin":7200}}}`)
	var wrapper wrapper
	err := json.Unmarshal(jsonData, &wrapper)
	assert.NoError(t, err, "failed to unmarshal wrapper")
	assert.Equal(t, "devStatus", wrapper.MSG.CMD, "CMD mismatch")
	var result devStatusResponse
	err = json.Unmarshal(wrapper.MSG.Data, &result)

	assert.NoError(t, err, "failed to unmarshal DevStatusResponse")
	assert.Equal(t, "On", result.OnOff.String())
	assert.Equal(t, "100%", result.Brightness.String())
	assert.Equal(t, "rgb(255, 0, 0)", result.Color.String())
	assert.Equal(t, "7200K", result.ColorKelvin.String())
}
