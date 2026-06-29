package bmc

import (
	"context"
	"fmt"
	"time"

	"github.com/bougou/go-ipmi"
)

type ipmiController struct {
	client *ipmi.Client
	cfg    Config
}

func NewIPMIController(cfg Config) (Controller, error) {
	client, err := ipmi.NewClient(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("create ipmi client: %w", err)
	}
	client.WithTimeout(5 * time.Second)
	if err := client.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("connect ipmi: %w", err)
	}
	return &ipmiController{client: client, cfg: cfg}, nil
}

func (c *ipmiController) Close() error {
	if c.client != nil {
		return c.client.Close(context.Background())
	}
	return nil
}

func (c *ipmiController) PowerOn(ctx context.Context) error {
	_, err := c.client.ChassisControl(ctx, ipmi.ChassisControlPowerUp)
	return err
}

func (c *ipmiController) PowerOff(ctx context.Context) error {
	_, err := c.client.ChassisControl(ctx, ipmi.ChassisControlPowerDown)
	return err
}

func (c *ipmiController) PowerRestart(ctx context.Context) error {
	_, err := c.client.ChassisControl(ctx, ipmi.ChassisControlPowerCycle)
	return err
}

func (c *ipmiController) PowerStatus(ctx context.Context) (PowerState, error) {
	status, err := c.client.GetChassisStatus(ctx)
	if err != nil {
		return PowerUnknown, fmt.Errorf("get chassis status: %w", err)
	}
	if status.PowerIsOn {
		return PowerOn, nil
	}
	return PowerOff, nil
}

func (c *ipmiController) SetBootDevice(ctx context.Context, device BootDevice) error {
	var selector ipmi.BootDeviceSelector
	switch device {
	case BootPXE:
		selector = ipmi.BootDeviceSelectorForcePXE
	case BootDisk:
		selector = ipmi.BootDeviceSelectorForceHardDrive
	case BootCDROM:
		selector = ipmi.BootDeviceSelectorForceCDROM
	case BootBIOS:
		selector = ipmi.BootDeviceSelectorForceBIOSSetup
	default:
		return fmt.Errorf("unknown boot device: %s", device)
	}
	// Use Legacy BIOS boot type; persist for all future boots
	return c.client.SetBootDevice(ctx, selector, ipmi.BIOSBootTypeLegacy, true)
}
