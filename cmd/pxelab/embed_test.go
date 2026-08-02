package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// embeddedSample 返回内嵌 bootdist 中第一个非目录条目及其内容，供测试比对
func embeddedSample(t *testing.T) (string, []byte) {
	t.Helper()
	srcFS, err := fs.Sub(embeddedBootFS, "bootdist")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	var name string
	var data []byte
	fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "" && !d.IsDir() {
			name = path
			data, _ = fs.ReadFile(srcFS, path)
		}
		return nil
	})
	if name == "" {
		t.Fatal("embedded bootdist 为空")
	}
	return name, data
}

func TestExtractBootFiles(t *testing.T) {
	root := t.TempDir()
	sample, sampleData := embeddedSample(t)

	// 首次释放：文件应被写入，且版本标记被创建
	extractBootFiles(root, true)

	dst := filepath.Join(root, sample)
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("首次释放后 %s 不存在: %v", sample, err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != string(sampleData) {
		t.Fatalf("首次释放内容不一致: err=%v", err)
	}
	marker, err := os.ReadFile(filepath.Join(root, bootdistMarker))
	if err != nil {
		t.Fatalf("首次释放后未写入版本标记: %v", err)
	}
	if strings.TrimSpace(string(marker)) != bootdistVersion {
		t.Fatalf("版本标记 = %q, 期望 %q", string(marker), bootdistVersion)
	}
}

func TestExtractBootFilesVersionMatchSkips(t *testing.T) {
	root := t.TempDir()
	sample, sampleData := embeddedSample(t)

	extractBootFiles(root, true) // 首次释放

	// 篡改已释放文件内容
	dst := filepath.Join(root, sample)
	if err := os.WriteFile(dst, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}

	// 版本一致时再次释放：不应覆盖用户修改
	extractBootFiles(root, true)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tampered" {
		t.Fatalf("版本一致时不应覆盖，内容被改写为 %q", string(got))
	}
	if string(sampleData) == "tampered" {
		t.Skip("内嵌样本恰为 tampered，跳过此断言")
	}
}

func TestExtractBootFilesVersionMismatchOverwrites(t *testing.T) {
	root := t.TempDir()
	sample, sampleData := embeddedSample(t)

	extractBootFiles(root, true) // 首次释放

	// 模拟旧版本标记 + 篡改文件
	if err := os.WriteFile(filepath.Join(root, bootdistMarker), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, sample)
	if err := os.WriteFile(dst, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}

	// 版本不一致：应覆盖恢复内嵌内容并更新标记
	extractBootFiles(root, true)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sampleData) {
		t.Fatalf("版本不一致时应覆盖为内嵌内容，实际 = %q", string(got))
	}
	marker, _ := os.ReadFile(filepath.Join(root, bootdistMarker))
	if strings.TrimSpace(string(marker)) != bootdistVersion {
		t.Fatalf("版本标记未更新: %q", string(marker))
	}
}

func TestExtractBootFilesPreservesUserFiles(t *testing.T) {
	root := t.TempDir()

	// 预置用户自定义文件 + 旧版本标记
	custom := filepath.Join(root, "my-custom.efi")
	if err := os.WriteFile(custom, []byte("user-data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, bootdistMarker), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}

	extractBootFiles(root, true) // 触发升级释放

	// 用户自定义文件必须保留
	got, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("升级释放后用户自定义文件被删除: %v", err)
	}
	if string(got) != "user-data" {
		t.Fatalf("用户自定义文件内容被篡改: %q", string(got))
	}
}

func TestExtractBootFilesMissingMarkerTriggersRelease(t *testing.T) {
	root := t.TempDir()
	sample, _ := embeddedSample(t)

	// 无标记（老版本场景）→ 应触发释放
	extractBootFiles(root, true)

	if _, err := os.Stat(filepath.Join(root, sample)); err != nil {
		t.Fatalf("无标记时应触发释放, %s 不存在: %v", sample, err)
	}
}

// TestExtractBootFilesManualModeNoOverwrite 手动模式（autoUpdate=false）：
// 已存在的文件（含用户篡改的）不得被覆盖，缺失文件应被补发。
func TestExtractBootFilesManualModeNoOverwrite(t *testing.T) {
	root := t.TempDir()
	sample, sampleData := embeddedSample(t)

	// 预置与内嵌同名的文件，内容被用户修改过
	dst := filepath.Join(root, sample)
	if err := os.WriteFile(dst, []byte("user-customized"), 0644); err != nil {
		t.Fatal(err)
	}

	// 手动模式补发：不应覆盖用户文件
	extractBootFiles(root, false)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "user-customized" {
		t.Fatalf("手动模式不应覆盖已存在文件，实际 = %q", string(got))
	}
	if string(sampleData) == "user-customized" {
		t.Skip("内嵌样本恰为 user-customized，跳过此断言")
	}

	// 手动模式不应写入版本标记
	if _, err := os.Stat(filepath.Join(root, bootdistMarker)); err == nil {
		t.Fatal("手动模式不应写入版本标记")
	}
}

// TestExtractBootFilesManualModeFillsMissing 手动模式（autoUpdate=false）：
// 缺失的内嵌文件应被补发（首次运行 / 用户删除默认文件后）。
func TestExtractBootFilesManualModeFillsMissing(t *testing.T) {
	root := t.TempDir()
	sample, sampleData := embeddedSample(t)

	// 空目录首次补发：文件应被写入
	extractBootFiles(root, false)

	dst := filepath.Join(root, sample)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("手动模式首次应补发缺失文件 %s: %v", sample, err)
	}
	if string(got) != string(sampleData) {
		t.Fatalf("手动模式补发内容不一致: err=%v", err)
	}
}
