package bmc

import "context"

type PowerState string

const (
	PowerOn      PowerState = "on"
	PowerOff     PowerState = "off"
	PowerUnknown PowerState = "unknown"
)

type BootDevice string

const (
	BootPXE   BootDevice = "pxe"
	BootDisk  BootDevice = "disk"
	BootCDROM BootDevice = "cdrom"
	BootBIOS  BootDevice = "bios"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Redfish  bool
}

type Controller interface {
	PowerOn(ctx context.Context) error
	PowerOff(ctx context.Context) error
	PowerRestart(ctx context.Context) error
	PowerStatus(ctx context.Context) (PowerState, error)
	SetBootDevice(ctx context.Context, device BootDevice) error
	Close() error
}
