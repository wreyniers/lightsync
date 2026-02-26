package lights

import "context"

type Controller interface {
	Brand() Brand
	Discover(ctx context.Context) ([]Device, error)
	Connect(ctx context.Context, deviceID string) error
	SetState(ctx context.Context, deviceID string, state DeviceState) error
	GetState(ctx context.Context, deviceID string) (DeviceState, error)
	TurnOn(ctx context.Context, deviceID string) error
	TurnOff(ctx context.Context, deviceID string) error
	Close() error
}
