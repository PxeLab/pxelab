package updatecheck

import "time"

// ReleaseInfo 来自官网 version.json 的版本发布信息
type ReleaseInfo struct {
	LatestVersion    string `json:"latest_version"`
	ReleaseDate      string `json:"release_date"`
	ReleaseNotesURL  string `json:"release_notes_url"`
	DownloadURL      string `json:"download_url"`
	ChecksumsURL     string `json:"checksums_url"`
	MinUpgradeVersion string `json:"min_upgrade_version"`
}

// CheckResult 版本检查结果
type CheckResult struct {
	CurrentVersion  string       `json:"current_version"`
	LatestVersion   string       `json:"latest_version"`
	UpdateAvailable bool         `json:"update_available"`
	ReleaseInfo     *ReleaseInfo `json:"release_info,omitempty"`
	CheckedAt       string       `json:"checked_at"`
	Error           string       `json:"error,omitempty"`
}

// DownloadResult 下载结果
type DownloadResult struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Version  string `json:"version"`
}

// PlatformAsset GitHub Release 对应平台的资源文件名
type PlatformAsset struct {
	OS   string // "linux", "windows", "darwin"
	Arch string // "amd64", "arm64"
	Ext  string // ".tar.gz", ".zip"
}

// AssetName 返回对应平台的 release asset 名
func AssetName(version, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return "pxelab_" + version + "_" + goos + "_" + goarch + ext
}

// IsExpired 检查检查结果是否过期（超过指定时长）
func (r *CheckResult) IsExpired(d time.Duration) bool {
	if r == nil {
		return true
	}
	t, err := time.Parse(time.RFC3339, r.CheckedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > d
}
