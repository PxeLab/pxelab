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
	Close() error
	Migrate() error
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
