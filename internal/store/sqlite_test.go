package store

import (
	"context"
	"testing"
	"time"

	"github.com/pxego/pxego/internal/models"
)

func setupDB(t *testing.T) Interface {
	t.Helper()
	st, err := NewSQLite("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSQLiteHostCRUD(t *testing.T) {
	st := setupDB(t)
	defer st.Close()
	ctx := context.Background()

	// Create
	host := &models.Host{
		ID:   "test-1",
		Name: "test-host",
		MAC:  "00:11:22:33:44:55",
		IP:   "192.168.1.100",
	}
	if err := st.CreateHost(ctx, host); err != nil {
		t.Fatal(err)
	}

	// Get
	got, err := st.GetHost(ctx, "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-host" {
		t.Errorf("expected name 'test-host', got %s", got.Name)
	}
	if got.MAC != "00:11:22:33:44:55" {
		t.Errorf("expected MAC '00:11:22:33:44:55', got %s", got.MAC)
	}

	// Get by MAC
	gotByMAC, err := st.GetHostByMAC(ctx, "00:11:22:33:44:55")
	if err != nil {
		t.Fatal(err)
	}
	if gotByMAC.ID != "test-1" {
		t.Errorf("expected host test-1, got %s", gotByMAC.ID)
	}

	// List
	hosts, total, err := st.ListHosts(ctx, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(hosts))
	}

	// Update
	host.Name = "updated-host"
	if err := st.UpdateHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetHost(ctx, "test-1")
	if got.Name != "updated-host" {
		t.Errorf("expected 'updated-host', got %s", got.Name)
	}

	// Search
	hosts, total, err = st.ListHosts(ctx, "updated", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("search: expected total 1, got %d", total)
	}

	// Delete
	if err := st.DeleteHost(ctx, "test-1"); err != nil {
		t.Fatal(err)
	}
	_, err = st.GetHost(ctx, "test-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestSQLiteProfileCRUD(t *testing.T) {
	st := setupDB(t)
	defer st.Close()
	ctx := context.Background()

	// Create default profile
	p1 := &models.Profile{
		ID:        "p1",
		Name:      "Default Profile",
		IsDefault: true,
	}
	if err := st.CreateProfile(ctx, p1); err != nil {
		t.Fatal(err)
	}

	// Create second profile (should unset default)
	p2 := &models.Profile{
		ID:        "p2",
		Name:      "New Default",
		IsDefault: true,
	}
	if err := st.CreateProfile(ctx, p2); err != nil {
		t.Fatal(err)
	}

	// Check only one default
	def, err := st.GetDefaultProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != "p2" {
		t.Errorf("expected p2 as default, got %s", def.ID)
	}

	// List
	profiles, err := st.ListProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestSQLiteEventCRUD(t *testing.T) {
	st := setupDB(t)
	defer st.Close()
	ctx := context.Background()

	// Create events
	events := []*models.Event{
		{ID: "e1", Type: models.EventDHCP, Level: models.EventInfo, Message: "DHCP request", Timestamp: time.Now()},
		{ID: "e2", Type: models.EventTFTP, Level: models.EventInfo, Message: "TFTP read", Timestamp: time.Now()},
		{ID: "e3", Type: models.EventDHCP, Level: models.EventWarn, Message: "DHCP warning", Timestamp: time.Now()},
	}
	for _, e := range events {
		if err := st.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	// List all
	all, total, err := st.ListEvents(ctx, EventFilter{Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 events, got %d", len(all))
	}

	// Filter by type
	dhcpEvents, total, err := st.ListEvents(ctx, EventFilter{Type: "DHCP", Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected 2 DHCP events, got %d", total)
	}
	if len(dhcpEvents) != 2 {
		t.Errorf("expected 2 DHCP events, got %d", len(dhcpEvents))
	}

	// Prune
	if err := st.PruneEvents(ctx, time.Now().Add(1*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	afterPrune, total, _ := st.ListEvents(ctx, EventFilter{Page: 1, Size: 10})
	if total != 0 {
		t.Errorf("expected 0 events after prune, got %d", total)
	}
	_ = afterPrune
}
