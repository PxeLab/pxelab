package store

import (
	"context"

	"github.com/pxego/pxego/internal/models"
)

type Interface interface {
	HostStore
	ProfileStore
	EventStore
	LeaseStore
	NetbootOverlayStore
	AnswerTemplateStore
	InstallTaskStore
	DNSRecordStore
	BlacklistStore
	WhitelistStore
	UnauthorizedDeviceStore
	BMCConfigStore
	DHCPReservationStore
	Close() error
	Migrate() error
	Seed() error
}

type HostStore interface {
	ListHosts(ctx context.Context, search string, page, size int) ([]models.Host, int64, error)
	GetHost(ctx context.Context, id string) (*models.Host, error)
	GetHostByMAC(ctx context.Context, mac string) (*models.Host, error)
	CreateHost(ctx context.Context, host *models.Host) error
	UpdateHost(ctx context.Context, host *models.Host) error
	DeleteHost(ctx context.Context, id string) error
}

type ProfileStore interface {
	ListProfiles(ctx context.Context) ([]models.Profile, error)
	GetDefaultProfile(ctx context.Context) (*models.Profile, error)
	GetProfile(ctx context.Context, id string) (*models.Profile, error)
	CreateProfile(ctx context.Context, profile *models.Profile) error
	UpdateProfile(ctx context.Context, profile *models.Profile) error
	DeleteProfile(ctx context.Context, id string) error
}

type EventStore interface {
	ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, int64, error)
	CreateEvent(ctx context.Context, event *models.Event) error
	PruneEvents(ctx context.Context, before int64) error
}

type EventFilter struct {
	Type   string
	Level  string
	Search string
	From   int64
	To     int64
	Page   int
	Size   int
}

type LeaseStore interface {
	ListLeases(ctx context.Context) ([]models.Lease, error)
	CreateLease(ctx context.Context, lease *models.Lease) error
	DeleteLease(ctx context.Context, mac string) error
	PruneLeases(ctx context.Context) error
}

type NetbootOverlayStore interface {
	ListNetbootOverlays(ctx context.Context) ([]models.NetbootOverlay, error)
	GetNetbootOverlay(ctx context.Context, distroName string) (*models.NetbootOverlay, error)
	UpsertNetbootOverlay(ctx context.Context, o *models.NetbootOverlay) error
	DeleteNetbootOverlay(ctx context.Context, distroName string) error
}

type AnswerTemplateStore interface {
	ListAnswerTemplates(ctx context.Context) ([]models.AnswerTemplate, error)
	GetAnswerTemplate(ctx context.Context, id uint) (*models.AnswerTemplate, error)
	CreateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error
	UpdateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error
	DeleteAnswerTemplate(ctx context.Context, id uint) error
	AnswerTemplateVersionStore
}

type AnswerTemplateVersionStore interface {
	ListAnswerTemplateVersions(ctx context.Context, templateID uint) ([]models.AnswerTemplateVersion, error)
	GetAnswerTemplateVersion(ctx context.Context, templateID uint, version int) (*models.AnswerTemplateVersion, error)
	CreateAnswerTemplateVersion(ctx context.Context, v *models.AnswerTemplateVersion) error
	DeleteAnswerTemplateVersions(ctx context.Context, templateID uint) error
}

type InstallTaskStore interface {
	ListInstallTasks(ctx context.Context) ([]models.InstallTask, error)
	GetInstallTask(ctx context.Context, id string) (*models.InstallTask, error)
	CreateInstallTask(ctx context.Context, t *models.InstallTask) error
	UpdateInstallTask(ctx context.Context, t *models.InstallTask) error
	DeleteInstallTask(ctx context.Context, id string) error
	GetInstallTaskByHostMAC(ctx context.Context, mac string) (*models.InstallTask, error)
}

type DNSRecordStore interface {
	ListDNSRecords(ctx context.Context) ([]models.DNSRecord, error)
	GetDNSRecord(ctx context.Context, id uint) (*models.DNSRecord, error)
	CreateDNSRecord(ctx context.Context, r *models.DNSRecord) error
	UpdateDNSRecord(ctx context.Context, r *models.DNSRecord) error
	DeleteDNSRecord(ctx context.Context, id uint) error
	FindDNSRecords(ctx context.Context, name string, recordType string, subnet string) ([]models.DNSRecord, error)
}

type BlacklistStore interface {
	ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error)
	CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error
	DeleteBlacklist(ctx context.Context, id uint) error
	IsBlacklisted(ctx context.Context, mac string) (bool, error)
}

type WhitelistStore interface {
	ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error)
	CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error
	DeleteWhitelist(ctx context.Context, id uint) error
	IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error)
}

type UnauthorizedDeviceStore interface {
	ListUnauthorizedDevices(ctx context.Context) ([]models.UnauthorizedDevice, error)
	UpsertUnauthorizedDevice(ctx context.Context, mac, subnetCIDR, reason string) error
	DeleteUnauthorizedDevice(ctx context.Context, id uint) error
	DeleteUnauthorizedDeviceByMAC(ctx context.Context, mac, subnetCIDR string) error
	DeleteAllUnauthorizedDevices(ctx context.Context) error
}

type BMCConfigStore interface {
	ListBMCConfigs(ctx context.Context) ([]models.BMCConfig, error)
	GetBMCConfig(ctx context.Context, id int64) (*models.BMCConfig, error)
	CreateBMCConfig(ctx context.Context, cfg *models.BMCConfig) error
	UpdateBMCConfig(ctx context.Context, cfg *models.BMCConfig) error
	DeleteBMCConfig(ctx context.Context, id int64) error
}

type DHCPReservationStore interface {
	ListDHCPReservations(ctx context.Context, subnetCIDR string) ([]models.DHCPReservation, error)
	GetDHCPReservation(ctx context.Context, id uint) (*models.DHCPReservation, error)
	GetDHCPReservationByMAC(ctx context.Context, subnetCIDR, mac string) (*models.DHCPReservation, error)
	GetDHCPReservationByIP(ctx context.Context, subnetCIDR, ip string) (*models.DHCPReservation, error)
	CreateDHCPReservation(ctx context.Context, r *models.DHCPReservation) error
	UpdateDHCPReservation(ctx context.Context, r *models.DHCPReservation) error
	DeleteDHCPReservation(ctx context.Context, id uint) error
}
