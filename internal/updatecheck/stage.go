package updatecheck

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SetApplyFunc 注册自动安装回调；由宿主进程提供（负责替换二进制并重启）。
func (c *Checker) SetApplyFunc(fn func(*StagedUpdate) (any, error)) {
	c.applyFn = fn
}

// 已知有问题的发布包：0.2.0 的构建产物缺少前端 assets（Linux 大小写路径 bug），
// 自动安装会得到一个能启动但 UI 404 的版本。0.2.1 发布后可移除。
var blockedReleases = map[string]string{
	"0.2.0": "0.2.0 安装包缺少前端静态资源，自动安装已阻止；请升级到 0.2.1",
}

// Apply 校验并解压更新，然后调用宿主的安装回调。
func (c *Checker) Apply() (any, error) {
	c.mu.RLock()
	latest := ""
	if c.lastResult != nil {
		latest = strings.TrimPrefix(c.lastResult.LatestVersion, "v")
	}
	c.mu.RUnlock()
	if msg, blocked := blockedReleases[latest]; blocked {
		return nil, fmt.Errorf("%s", msg)
	}

	staged, err := c.Stage()
	if err != nil {
		return nil, err
	}
	if msg, blocked := blockedReleases[staged.Version]; blocked {
		return nil, fmt.Errorf("%s", msg)
	}
	if c.applyFn == nil {
		return nil, fmt.Errorf("当前运行模式不支持自动安装，请手动替换二进制")
	}
	return c.applyFn(staged)
}

// StagedUpdate 是已下载、校验并解压完成的待安装更新。
type StagedUpdate struct {
	Version     string `json:"version"`
	ArchivePath string `json:"archive_path"`
	BinaryPath  string `json:"binary_path"`
	SHA256      string `json:"sha256"`
}

// Stage 下载更新包、校验 SHA-256，并从中解出可执行文件。
func (c *Checker) Stage() (*StagedUpdate, error) {
	dl, err := c.Download()
	if err != nil {
		return nil, err
	}

	sums, err := c.fetchChecksums(dl.Version)
	if err != nil {
		return nil, fmt.Errorf("获取校验和失败: %w", err)
	}
	expected, ok := sums[dl.FileName]
	if !ok || expected == "" {
		return nil, fmt.Errorf("release 校验和里没有 %s", dl.FileName)
	}

	actual, err := fileSHA256(dl.FilePath)
	if err != nil {
		return nil, fmt.Errorf("计算更新包校验和失败: %w", err)
	}
	if !strings.EqualFold(actual, expected) {
		return nil, fmt.Errorf("更新包校验失败: 期望 %s，实际 %s", expected, actual)
	}

	binaryPath, err := extractBinary(dl.FilePath, dl.Version, c.dataDir)
	if err != nil {
		return nil, fmt.Errorf("解压更新包失败: %w", err)
	}

	return &StagedUpdate{
		Version:     dl.Version,
		ArchivePath: dl.FilePath,
		BinaryPath:  binaryPath,
		SHA256:      actual,
	}, nil
}

// fetchChecksums 通过 GitHub API 获取 release 的 checksums 文件并解析。
func (c *Checker) fetchChecksums(version string) (map[string]string, error) {
	assetName := "pxelab_" + version + "_checksums.txt"
	url, err := c.resolveAssetURL(version, assetName)
	if err != nil {
		// 回退到直连地址，兼容 GitHub API 不可用但下载站可用的环境
		url = fmt.Sprintf("%s/%s/releases/download/v%s/%s", c.downloadBase, c.repo, version, assetName)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PxeLab/"+c.currentVersion)
	if strings.Contains(url, "/releases/assets/") {
		req.Header.Set("Accept", "application/octet-stream")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("校验和下载返回 HTTP %d", resp.StatusCode)
	}

	return parseChecksums(io.LimitReader(resp.Body, 1<<20))
}

func parseChecksums(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		result[name] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("校验和文件为空")
	}
	return result, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinary 从 zip/tar.gz 中取出平台对应的 pxelab 可执行文件。
func extractBinary(archivePath, version, dataDir string) (string, error) {
	updatesDir := filepath.Join(dataDir, "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return "", err
	}

	exeName := "pxelab"
	if runtime.GOOS == "windows" {
		exeName = "pxelab.exe"
	}
	dest := filepath.Join(updatesDir, "pxelab-"+version+filepath.Ext(exeName))

	tmp, err := os.Create(dest + ".tmp")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		tmp.Close()
		if !success {
			os.Remove(dest + ".tmp")
		}
	}()

	var copyErr error
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		copyErr = extractFromZip(archivePath, exeName, tmp)
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"):
		copyErr = extractFromTarGz(archivePath, exeName, tmp)
	default:
		copyErr = fmt.Errorf("不支持的更新包格式: %s", filepath.Base(archivePath))
	}
	if copyErr != nil {
		return "", copyErr
	}

	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(dest+".tmp", 0755); err != nil {
		return "", err
	}
	if err := os.Rename(dest+".tmp", dest); err != nil {
		return "", err
	}
	success = true
	return dest, nil
}

func extractFromZip(archivePath, exeName string, dst io.Writer) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, entry := range zr.File {
		if filepath.Base(entry.Name) != exeName {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dst, io.LimitReader(rc, 512<<20))
		rc.Close()
		return copyErr
	}
	return fmt.Errorf("更新包里没有 %s", exeName)
}

func extractFromTarGz(archivePath, exeName string, dst io.Writer) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != exeName {
			continue
		}
		_, copyErr := io.Copy(dst, io.LimitReader(tr, 512<<20))
		return copyErr
	}
	return fmt.Errorf("更新包里没有 %s", exeName)
}
