package models

import "time"

// PxeBootRecord 记录“哪些机器来 PXE 引导过、大致走了哪条路”，
// 供管理员从引导记录一键认领为主机（未认领的机器不会触发任何基线）。
type PxeBootRecord struct {
	MAC           string    `json:"mac" gorm:"primaryKey;column:mac"`
	IP            string    `json:"ip"`
	Loader        string    `json:"loader"`       // ipxe | pxelinux | grub | (空=保留旧值)
	LastContext   string    `json:"last_context"` // default-menu / host-profile:<name> / failsafe / install-task:<distro>
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	Count         int       `json:"count"`
	ClaimedHostID *string   `json:"claimed_host_id"`
}

func (PxeBootRecord) TableName() string { return "pxe_boot_records" }
