package bmc

import "fmt"

func NewController(cfg Config) (Controller, error) {
	if cfg.Redfish {
		return nil, fmt.Errorf("Redfish not yet implemented")
	}
	return NewIPMIController(cfg)
}
