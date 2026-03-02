package govee

import "errors"

var (
	ErrInvalidVersionFormat = errors.New("invalid version format")
	ErrNoDeviceFound        = errors.New("no device found")
)
