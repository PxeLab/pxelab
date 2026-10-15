package osimage

import (
	"runtime"
	"strings"
	"testing"
)

// 解析层测试（无 dism、无 WIM，任意平台可跑）。真机上的 cmdlet 执行路径
// 属于硬件验证范围（见 docs/design/windows-install-roadmap.md §1.3）。

const testWimInfoJSON = `[{"ImageIndex":1,"ImageName":"Windows Setup","ImageDescription":"Microsoft Windows Operating System","ImageSize":234567890},{"ImageIndex":2,"ImageName":"Microsoft Windows PE (x64) - Recovery","ImageDescription":"","ImageSize":56789012}]`

func TestParseWIMIndexes(t *testing.T) {
	idxs, err := parseWIMIndexes(testWimInfoJSON)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(idxs) != 2 {
		t.Fatalf("len = %d, want 2", len(idxs))
	}
	if idxs[0].Index != 1 || idxs[0].Name != "Windows Setup" || idxs[0].Size != 234567890 {
		t.Fatalf("idx[0] = %+v", idxs[0])
	}
	if idxs[1].Index != 2 || !strings.Contains(idxs[1].Name, "Recovery") {
		t.Fatalf("idx[1] = %+v", idxs[1])
	}
}

func TestParseWIMIndexesSingleObject(t *testing.T) {
	// ConvertTo-Json 对单个对象不输出数组 —— 兼容为单元素列表
	idxs, err := parseWIMIndexes(`{"ImageIndex":1,"ImageName":"Windows Setup","ImageDescription":"","ImageSize":42}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(idxs) != 1 || idxs[0].Index != 1 || idxs[0].Size != 42 {
		t.Fatalf("idxs = %+v", idxs)
	}
}

func TestParseWIMIndexesEmpty(t *testing.T) {
	// 无镜像 / 无第三方驱动时 cmdlet 无输出
	idxs, err := parseWIMIndexes("")
	if err != nil || len(idxs) != 0 {
		t.Fatalf("idxs = %+v, err = %v", idxs, err)
	}
}

func TestParseWIMIndexesInvalid(t *testing.T) {
	if _, err := parseWIMIndexes("not json"); err == nil {
		t.Fatal("非 JSON 输出应报错")
	}
}

const testWimDriversJSON = `[{"ProviderName":"Intel","ClassName":"Net","Driver":"oem1.inf","Version":"12.3.4.5","BootCritical":false},{"ProviderName":"LSI","ClassName":"SCSIAdapter","Driver":"oem2.inf","Version":"2.0.1.0","BootCritical":true}]`

func TestParseWIMDrivers(t *testing.T) {
	drivers, err := parseWIMDrivers(testWimDriversJSON)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(drivers) != 2 {
		t.Fatalf("len = %d, want 2", len(drivers))
	}
	if drivers[0].ProviderName != "Intel" || drivers[0].Driver != "oem1.inf" || drivers[0].ClassName != "Net" {
		t.Fatalf("drivers[0] = %+v", drivers[0])
	}
	if !drivers[1].BootCritical {
		t.Fatalf("drivers[1].BootCritical = %+v", drivers[1])
	}
}

func TestParseWIMDriversSingleObject(t *testing.T) {
	drivers, err := parseWIMDrivers(`{"ProviderName":"Intel","ClassName":"Net","Driver":"oem1.inf","Version":"1.0","BootCritical":true}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(drivers) != 1 || drivers[0].ProviderName != "Intel" || !drivers[0].BootCritical {
		t.Fatalf("drivers = %+v", drivers)
	}
}

func TestParseWIMDriversEmpty(t *testing.T) {
	drivers, err := parseWIMDrivers("  \n")
	if err != nil || len(drivers) != 0 {
		t.Fatalf("drivers = %+v, err = %v", drivers, err)
	}
}

func TestErrWIMUnsupportedMessage(t *testing.T) {
	if !strings.Contains(ErrWIMUnsupported.Error(), "Windows host") {
		t.Fatalf("错误信息应说明需要 Windows 主机: %v", ErrWIMUnsupported)
	}
}

func TestWIMCapabilitiesMatchesPlatform(t *testing.T) {
	cap := WIMCapabilities()
	if cap.Platform != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", cap.Platform, runtime.GOOS)
	}
	wantSupported := runtime.GOOS == "windows"
	if cap.OfflineDriverInjection != wantSupported {
		t.Fatalf("offline_driver_injection = %v, want %v (GOOS=%s)", cap.OfflineDriverInjection, wantSupported, runtime.GOOS)
	}
	if !cap.OfflineDriverInjection && cap.Reason == "" {
		t.Fatal("不支持时应给出原因")
	}
	// 非 Windows 需明确指向替代方案
	if runtime.GOOS != "windows" && !strings.Contains(strings.ToLower(cap.Reason), "pe-stage") {
		t.Fatalf("不支持的原因应指向 PE 阶段运行时加载: %q", cap.Reason)
	}
}
