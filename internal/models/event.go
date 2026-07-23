package models

import "time"

type EventType string

const (
	EventDHCP    EventType = "DHCP"
	EventTFTP    EventType = "TFTP"
	EventHTTP    EventType = "HTTP"
	EventBoot    EventType = "BOOT"
	EventIPMI    EventType = "IPMI"
	EventDNS     EventType = "DNS"
	EventOSImage EventType = "OS_IMAGE"
)

type EventLevel string

const (
	EventInfo  EventLevel = "INFO"
	EventWarn  EventLevel = "WARN"
	EventError EventLevel = "ERROR"
)

type Event struct {
	ID        string         `json:"id" gorm:"primaryKey"`
	Type      EventType      `json:"type" gorm:"index"`
	Level     EventLevel     `json:"level" gorm:"index"`
	Message   string         `json:"message"`
	MAC       *string        `json:"mac,omitempty"`
	IP        *string        `json:"ip,omitempty"`
	Detail    map[string]any `json:"detail,omitempty" gorm:"-:all"`
	Timestamp time.Time      `json:"timestamp" gorm:"index"`
}
