package bmc

import (
	"context"
	"fmt"
	"time"

	"github.com/bougou/go-ipmi"
)

type ProbeResult struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Serial string `json:"serial"`
	MAC    string `json:"mac"`
}

// ProbeDevice connects to a BMC and attempts to read FRU info.
// Returns whatever fields were found; empty strings for missing data.
// This is best-effort — many BMCs have incomplete FRU data.
func ProbeDevice(ctx context.Context, cfg Config) (*ProbeResult, error) {
	client, err := ipmi.NewClient(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("create ipmi client: %w", err)
	}
	client.WithTimeout(5 * time.Second)
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect ipmi: %w", err)
	}
	defer client.Close(ctx)

	result := &ProbeResult{}

	// ── FRU data ──
	frus, err := client.GetFRUs(ctx)
	if err != nil {
		// FRU not available — return empty result, not an error
		// (BMC is reachable but doesn't support FRU inventory)
		return result, nil
	}

	for _, fru := range frus {
		if !fru.Present() {
			continue
		}
		if fru.BoardInfoArea != nil {
			if len(fru.BoardInfoArea.Manufacturer) > 0 {
				result.Vendor = string(fru.BoardInfoArea.Manufacturer)
			}
			if len(fru.BoardInfoArea.ProductName) > 0 {
				result.Model = string(fru.BoardInfoArea.ProductName)
				if result.Name == "" {
					result.Name = string(fru.BoardInfoArea.ProductName)
				}
			}
			if len(fru.BoardInfoArea.SerialNumber) > 0 {
				result.Serial = string(fru.BoardInfoArea.SerialNumber)
			}
		}
		if fru.ProductInfoArea != nil {
			if len(fru.ProductInfoArea.Name) > 0 {
				result.Name = string(fru.ProductInfoArea.Name)
			}
			if result.Vendor == "" && len(fru.ProductInfoArea.Manufacturer) > 0 {
				result.Vendor = string(fru.ProductInfoArea.Manufacturer)
			}
			if result.Model == "" && len(fru.ProductInfoArea.PartModel) > 0 {
				result.Model = string(fru.ProductInfoArea.PartModel)
			}
			if result.Serial == "" && len(fru.ProductInfoArea.SerialNumber) > 0 {
				result.Serial = string(fru.ProductInfoArea.SerialNumber)
			}
		}
	}

	// ── MAC from LAN config ──
	lanCfg, err := client.GetLanConfig(ctx, 1)
	if err == nil && lanCfg != nil {
		result.MAC = lanCfg.MAC.String()
	}

	return result, nil
}
