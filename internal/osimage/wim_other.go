//go:build !windows

package osimage

// 非 Windows 平台：真正的离线 WIM 驱动注入需要 Windows 的 driver store 注册
// 语义（dism），wimlib 只能做文件级挂载无法注册驱动，故一律返回
// ErrWIMUnsupported；调用方应提示用户改用 PE 阶段运行时驱动加载（借鉴项 2）。

func MountWIM(wimPath, mountDir string, index int) error {
	return ErrWIMUnsupported
}

func AddDriverToWIM(mountDir, driverDir string, recurse bool) error {
	return ErrWIMUnsupported
}

func UnmountWIM(mountDir string, commit bool) error {
	return ErrWIMUnsupported
}

func ListWIMDrivers(mountDir string) ([]WIMDriver, error) {
	return nil, ErrWIMUnsupported
}

func CleanupWIM() error {
	return ErrWIMUnsupported
}

func GetWIMInfo(wimPath string) ([]WIMIndex, error) {
	return nil, ErrWIMUnsupported
}
