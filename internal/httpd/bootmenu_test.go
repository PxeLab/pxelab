package httpd

import (
	"context"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

func strptr(s string) *string { return &s }

// seedWDSHostProfile 建一个绑定 wds 引导项的 Profile + 主机，返回主机 MAC。
func seedWDSHostProfile(t *testing.T, st store.Interface, ctx context.Context) string {
	t.Helper()
	prof := &models.Profile{Name: "Win", IsDefault: true}
	if err := prof.SetMenu(&models.BootMenu{Entries: []models.MenuEntry{{
		Label: "Windows 11",
		Type:  "wds",
		URL:   strptr("http://10.0.0.10:8080/netboot/menu/wimboot"),
		WIM:   strptr("http://10.0.0.10:8080/boot/isos/win11"),
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProfile(ctx, prof); err != nil {
		t.Fatal(err)
	}
	host := &models.Host{Name: "pc1", MAC: "00:11:22:33:44:55", ProfileID: &prof.ID}
	if err := st.CreateHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	return host.MAC
}

// TestGenerateBootMenuInjectsWinpeShellAnswer：主机有活跃安装任务且 overlay
// answer_type=winpeshl 时，wds 引导项应带上 install.bat + winpeshl.ini 注入行。
func TestGenerateBootMenuInjectsWinpeShellAnswer(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	mac := seedWDSHostProfile(t, st, ctx)

	ov := &models.NetbootOverlay{DistroName: "Windows", Enabled: true}
	if err := ov.SetVersionOverrides([]models.VersionOverride{{Codename: "w11", AnswerType: "winpeshl"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertNetbootOverlay(ctx, ov); err != nil {
		t.Fatal(err)
	}
	host, _ := st.GetHostByMAC(ctx, mac)
	task := &models.InstallTask{
		ID: "task-1", HostID: host.ID, DistroName: "Windows",
		VersionCodename: "w11", Status: "pending",
	}
	if err := st.CreateInstallTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	out, err := generateBootMenu(config.DefaultConfig(), st, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "initrd -n install.bat http://10.0.0.10:8080/api/v1/netboot/answer/task-1") {
		t.Fatalf("expected install.bat answer injection, got:\n%s", out)
	}
	if !strings.Contains(out, "initrd -n winpeshl.ini http://10.0.0.10:8080/api/v1/netboot/answer/task-1?file=winpeshl.ini") {
		t.Fatalf("expected winpeshl.ini answer injection, got:\n%s", out)
	}
}

// TestGenerateBootMenuNoTaskNoInjection：无安装任务时 wds 引导项不应注入应答。
func TestGenerateBootMenuNoTaskNoInjection(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	mac := seedWDSHostProfile(t, st, ctx)

	out, err := generateBootMenu(config.DefaultConfig(), st, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "/api/v1/netboot/answer/") {
		t.Fatalf("no task => no answer injection, got:\n%s", out)
	}
	if !strings.Contains(out, "kernel http://10.0.0.10:8080/netboot/menu/wimboot") {
		t.Fatalf("expected plain wimboot kernel line, got:\n%s", out)
	}
}

// TestGenerateBootMenuFailedTaskLocked：最新任务 failed 时锁定，不注入应答。
func TestGenerateBootMenuFailedTaskLocked(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	mac := seedWDSHostProfile(t, st, ctx)

	host, _ := st.GetHostByMAC(ctx, mac)
	task := &models.InstallTask{
		ID: "task-f", HostID: host.ID, DistroName: "Windows",
		VersionCodename: "w11", Status: "failed",
	}
	if err := st.CreateInstallTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	out, err := generateBootMenu(config.DefaultConfig(), st, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "/api/v1/netboot/answer/") {
		t.Fatalf("failed task must be locked (no injection), got:\n%s", out)
	}
}
