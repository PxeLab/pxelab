package updatecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 默认官网版本信息地址
const DefaultVersionEndpoint = "https://www.pxelab.com/version.json"

// GitHub 仓库与 API 地址；下载更新时优先走 API，避免部分网络阻断 github.com 直连
const (
	defaultAPIBase      = "https://api.github.com"
	defaultDownloadBase = "https://github.com"
	defaultRepo         = "PxeLab/pxelab"
)

// Checker 版本检查器
type Checker struct {
	currentVersion string
	endpoint       string
	dataDir        string
	lastResult     *CheckResult
	mu             sync.RWMutex
	client         *http.Client
	downloadClient *http.Client
	apiBase        string
	downloadBase   string
	repo           string
}

// NewChecker 创建版本检查器
// currentVersion: 当前运行版本（由 ldflags 注入）
// dataDir: 数据目录（用于存放下载的更新包）
func NewChecker(currentVersion, dataDir string) *Checker {
	return &Checker{
		currentVersion: currentVersion,
		endpoint:       DefaultVersionEndpoint,
		dataDir:        dataDir,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		// 更新包可能较大，单独给下载更长的超时
		downloadClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		apiBase:      defaultAPIBase,
		downloadBase: defaultDownloadBase,
		repo:         defaultRepo,
	}
}

// WithEndpoint 设置自定义版本检查端点（用于测试）
func (c *Checker) WithEndpoint(endpoint string) *Checker {
	c.endpoint = endpoint
	return c
}

// Check 向官网发起版本检查，返回结果并更新缓存
func (c *Checker) Check() (*CheckResult, error) {
	slog.Debug("版本检查", "endpoint", c.endpoint, "current", c.currentVersion)

	resp, err := c.client.Get(c.endpoint)
	if err != nil {
		errMsg := fmt.Sprintf("版本检查请求失败: %v", err)
		slog.Warn("版本检查失败", "error", err)
		result := &CheckResult{
			CurrentVersion:  c.currentVersion,
			UpdateAvailable: false,
			CheckedAt:       time.Now().UTC().Format(time.RFC3339),
			Error:           errMsg,
		}
		c.mu.Lock()
		c.lastResult = result
		c.mu.Unlock()
		return result, fmt.Errorf("version check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var info ReleaseInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse version.json failed: %w", err)
	}

	if info.LatestVersion == "" {
		return nil, fmt.Errorf("version.json missing latest_version field")
	}

	// 语义化版本比较
	cmp := compareVersions(info.LatestVersion, c.currentVersion)
	available := cmp > 0

	releaseNotesURL := info.ReleaseNotesURL
	if releaseNotesURL == "" {
		releaseNotesURL = "https://github.com/pxelab/pxelab/releases/tag/v" + info.LatestVersion
	}

	downloadURL := info.DownloadURL
	if downloadURL == "" {
		downloadURL = "https://github.com/pxelab/pxelab/releases/tag/v" + info.LatestVersion
	}

	result := &CheckResult{
		CurrentVersion:  c.currentVersion,
		LatestVersion:   info.LatestVersion,
		UpdateAvailable: available,
		ReleaseInfo: &ReleaseInfo{
			LatestVersion:     info.LatestVersion,
			ReleaseDate:       info.ReleaseDate,
			ReleaseNotesURL:   releaseNotesURL,
			DownloadURL:       downloadURL,
			ChecksumsURL:      info.ChecksumsURL,
			MinUpgradeVersion: info.MinUpgradeVersion,
		},
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	c.mu.Lock()
	c.lastResult = result
	c.mu.Unlock()

	slog.Info("版本检查完成",
		"current", c.currentVersion,
		"latest", info.LatestVersion,
		"available", available,
	)

	return result, nil
}

// Result 返回最近一次缓存的检查结果（不会发起网络请求）
func (c *Checker) Result() *CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastResult
}

// Download 下载对应平台的最新版本二进制到 updates 目录
func (c *Checker) Download() (*DownloadResult, error) {
	c.mu.RLock()
	result := c.lastResult
	c.mu.RUnlock()

	if result == nil || result.ReleaseInfo == nil {
		return nil, fmt.Errorf("请先执行版本检查")
	}

	if !result.UpdateAvailable {
		return nil, fmt.Errorf("已是最新版本")
	}

	version := strings.TrimPrefix(result.LatestVersion, "v")
	assetName := AssetName(version, runtime.GOOS, runtime.GOARCH)

	updatesDir := filepath.Join(c.dataDir, "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return nil, fmt.Errorf("创建更新目录失败: %w", err)
	}

	destPath := filepath.Join(updatesDir, assetName)

	// 如果已存在，跳过
	if fi, err := os.Stat(destPath); err == nil && fi.Size() > 0 {
		slog.Info("更新包已存在", "path", destPath, "size", fi.Size())
		return &DownloadResult{
			FilePath: destPath,
			FileName: assetName,
			FileSize: fi.Size(),
			Version:  version,
		}, nil
	}

	var attempts []string

	// 首选 GitHub API：先解析 asset，再通过 asset API 下载。
	// 部分网络会阻断 github.com/releases/download 直连，但 api.github.com 可用。
	assetURL, resolveErr := c.resolveAssetURL(version, assetName)
	if resolveErr == nil {
		if err := c.downloadFile(assetURL, destPath); err == nil {
			fi, statErr := os.Stat(destPath)
			if statErr != nil {
				return nil, fmt.Errorf("下载完成但读取文件失败: %w", statErr)
			}
			slog.Info("更新包下载完成", "path", destPath, "size", fi.Size(), "via", "github-api")
			return &DownloadResult{
				FilePath: destPath,
				FileName: assetName,
				FileSize: fi.Size(),
				Version:  version,
			}, nil
		} else {
			attempts = append(attempts, err.Error())
		}
	} else {
		attempts = append(attempts, resolveErr.Error())
	}

	// 回退到直连地址（网络允许时更简单）
	directURL := fmt.Sprintf("%s/%s/releases/download/v%s/%s", c.downloadBase, c.repo, version, assetName)
	if err := c.downloadFile(directURL, destPath); err == nil {
		fi, statErr := os.Stat(destPath)
		if statErr != nil {
			return nil, fmt.Errorf("下载完成但读取文件失败: %w", statErr)
		}
		slog.Info("更新包下载完成", "path", destPath, "size", fi.Size(), "via", "direct")
		return &DownloadResult{
			FilePath: destPath,
			FileName: assetName,
			FileSize: fi.Size(),
			Version:  version,
		}, nil
	} else {
		attempts = append(attempts, err.Error())
	}

	return nil, fmt.Errorf("下载失败: %s。请检查网络/代理，或从 https://github.com/%s/releases 手动下载", strings.Join(attempts, "; "), c.repo)
}

// resolveAssetURL 查询 GitHub release，返回指定 asset 的 API 下载地址。
func (c *Checker) resolveAssetURL(version, assetName string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/tags/v%s", c.apiBase, c.repo, version)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("创建 release 查询请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PxeLab/"+c.currentVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询 release 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("release v%s 不存在", version)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("查询 release 失败: HTTP %d", resp.StatusCode)
	}

	var release struct {
		Assets []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("解析 release 失败: %w", err)
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return fmt.Sprintf("%s/repos/%s/releases/assets/%d", c.apiBase, c.repo, asset.ID), nil
		}
	}
	return "", fmt.Errorf("release v%s 缺少资源 %s", version, assetName)
}

// downloadFile 下载 url 到 destPath，先写 .tmp 再原子重命名。
func (c *Checker) downloadFile(url, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "PxeLab/"+c.currentVersion)
	if strings.Contains(url, "/releases/assets/") {
		// GitHub asset API 需要这个 Accept 才返回二进制内容
		req.Header.Set("Accept", "application/octet-stream")
	}

	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求下载地址失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回 HTTP %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("重命名文件失败: %w", err)
	}
	success = true
	return nil
}

// compareVersions 比较两个语义化版本号
// 返回: 1 (v1 > v2), 0 (v1 == v2), -1 (v1 < v2)
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")

	maxLen := len(p1)
	if len(p2) > maxLen {
		maxLen = len(p2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(p1) {
			n1, _ = strconv.Atoi(p1[i])
		}
		if i < len(p2) {
			n2, _ = strconv.Atoi(p2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}
