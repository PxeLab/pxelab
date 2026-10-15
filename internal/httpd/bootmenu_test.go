package httpd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/drivers"
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

	out, err := generateBootMenu(config.DefaultConfig(), st, nil, mac, "10.0.0.10:8080", ctx)
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

// TestGenerateBootMenuInitrdDriverDelivery：driver_delivery=initrd 且主机绑定驱动包、
// 任务走 winpeshl 时，wds 引导项应带上驱动文件的 iPXE initrd 注入行（每个文件一行），
// 且默认（http）方式不产生这些行。
func TestGenerateBootMenuInitrdDriverDelivery(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	mac := seedWDSHostProfile(t, st, ctx)

	// 驱动包：drivers/nic-intel/{e1d.inf, x64/e1d64.sys}
	root := t.TempDir()
	nic := filepath.Join(root, "drivers", "nic-intel", "x64")
	if err := os.MkdirAll(nic, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "drivers", "nic-intel", "e1d.inf"), []byte("inf"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nic, "e1d64.sys"), []byte("sys"), 0644); err != nil {
		t.Fatal(err)
	}
	bootFS := boot.NewBootFileServer(root)

	// 主机绑定驱动包
	host, _ := st.GetHostByMAC(ctx, mac)
	if err := host.SetDriverPacks([]string{"nic-intel"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateHost(ctx, host); err != nil {
		t.Fatal(err)
	}

	// 任务走 winpeshl（overlay answer_type）
	ov := &models.NetbootOverlay{DistroName: "Windows", Enabled: true}
	if err := ov.SetVersionOverrides([]models.VersionOverride{{Codename: "w11", AnswerType: "winpeshl"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertNetbootOverlay(ctx, ov); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID: "task-initrd", HostID: host.ID, DistroName: "Windows",
		VersionCodename: "w11", Arch: "amd64", Status: "pending",
	}
	if err := st.CreateInstallTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	// initrd 模式：菜单应含驱动文件的 initrd 行（注入名与 drivers.InjectName 一致）
	cfgInitrd := config.DefaultConfig()
	cfgInitrd.Netboot.DriverDelivery = "initrd"
	out, err := generateBootMenu(cfgInitrd, st, bootFS, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	infName := drivers.InjectName("nic-intel", "e1d.inf")
	sysName := drivers.InjectName("nic-intel", "x64/e1d64.sys")
	mustContain := []string{
		"initrd -n install.bat http://10.0.0.10:8080/api/v1/netboot/answer/task-initrd",
		"initrd -n " + infName + " http://10.0.0.10:8080/boot/drivers/nic-intel/e1d.inf",
		"initrd -n " + sysName + " http://10.0.0.10:8080/boot/drivers/nic-intel/x64/e1d64.sys",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Fatalf("initrd 模式菜单缺少 %q:\n%s", s, out)
		}
	}

	// 默认 http 模式：不得出现驱动 initrd 行
	outHTTP, err := generateBootMenu(config.DefaultConfig(), st, bootFS, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(outHTTP, infName) {
		t.Fatalf("http 模式不应注入驱动 initrd 行:\n%s", outHTTP)
	}
	if !strings.Contains(outHTTP, "initrd -n install.bat ") {
		t.Fatalf("http 模式仍应注入应答 initrd:\n%s", outHTTP)
	}
}

// TestGenerateBootMenuNoTaskNoInjection：无安装任务时 wds 引导项不应注入应答。
func TestGenerateBootMenuNoTaskNoInjection(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	mac := seedWDSHostProfile(t, st, ctx)

	out, err := generateBootMenu(config.DefaultConfig(), st, nil, mac, "10.0.0.10:8080", ctx)
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

	out, err := generateBootMenu(config.DefaultConfig(), st, nil, mac, "10.0.0.10:8080", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "/api/v1/netboot/answer/") {
		t.Fatalf("failed task must be locked (no injection), got:\n%s", out)
	}
}
