package govee

func ExampleDevice_TurnOn() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()
	device, _ := controller.DeviceByIP("192.168.1.100")
	if device != nil {
		_ = device.TurnOn()
	}
}
func ExampleDevice_TurnOff() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	if device != nil {
		_ = device.TurnOff()
	}
}

func ExampleDevice_Toggle() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	if device != nil {
		_ = device.Toggle()
	}
}

func ExampleDevice_SetBrightness() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	brightness := NewBrightness(75)
	if device != nil {
		_ = device.SetBrightness(brightness)
	}
}

func ExampleDevice_SetColor() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	color := NewColor(255, 0, 0)
	if device != nil {
		_ = device.SetColor(color)
	}
}

func ExampleDevice_SetColorKelvin() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	k := NewColorKelvin(3500)
	if device != nil {
		_ = device.SetColorKelvin(k)
	}
}

func ExampleDevice_RequestStatus() {
	controller := NewController(nil)
	go controller.Start()
	defer controller.Shutdown()

	device, _ := controller.DeviceByIP("192.168.1.100")
	if device != nil {
		_ = device.RequestStatus()
	}
}
