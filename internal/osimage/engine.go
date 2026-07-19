package osimage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type ISOMeta struct {
	Distro   string `json:"distro"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	Checksum string `json:"checksum"`
}

func DetectDistro(isoPath string, mountPoint string) (*ISOMeta, error) {
	meta := &ISOMeta{}
	checksum, err := computeSHA256(isoPath)
	if err == nil {
		meta.Checksum = checksum
	}

	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		return meta, nil
	}

	dirMap := make(map[string]bool)
	for _, e := range entries {
		dirMap[strings.ToLower(e.Name())] = e.IsDir()
	}

	if _, ok := dirMap[".disk"]; ok {
		meta.Distro = "ubuntu"
		if b, err := os.ReadFile(filepath.Join(mountPoint, ".disk", "info")); err == nil {
			meta.Version = parseUbuntuVersion(string(b))
		}
	} else if _, ok := dirMap["ubuntu"]; ok {
		meta.Distro = "ubuntu"
	} else if _, ok := dirMap["debian"]; ok {
		meta.Distro = "debian"
		if b, err := os.ReadFile(filepath.Join(mountPoint, "debian", "cdrom")); err == nil {
			meta.Version = parseDebianVersion(string(b))
		}
	} else if _, ok := dirMap["centos"]; ok || hasKey(dirMap, "centosstream") {
		meta.Distro = "centos"
		if b, err := os.ReadFile(filepath.Join(mountPoint, ".treeinfo")); err == nil {
			meta.Version = parseTreeInfoVersion(string(b))
		}
	} else if _, ok := dirMap["rocky"]; ok {
		meta.Distro = "rocky"
	} else if _, ok := dirMap["almalinux"]; ok {
		meta.Distro = "almalinux"
	} else if _, ok := dirMap["vmware"]; ok || hasESXIFiles(entries) {
		meta.Distro = "esxi"
		meta.Version = detectESXIVersion(entries)
	} else if hasWindowsFiles(entries) {
		meta.Distro = "windows"
	} else if _, ok := dirMap["live"]; ok {
		meta.Distro = "live"
	}

	meta.Arch = detectArch(mountPoint, entries, meta.Distro)
	return meta, nil
}

func hasKey(m map[string]bool, key string) bool {
	_, ok := m[key]
	return ok
}

func computeSHA256(path string) (string, error) {
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

func parseUbuntuVersion(s string) string {
	parts := strings.SplitN(strings.TrimSpace(s), " ", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return strings.TrimSpace(s)
}

func parseDebianVersion(s string) string {
	return strings.TrimSpace(s)
}

func parseTreeInfoVersion(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version=") {
			return strings.TrimPrefix(line, "version=")
		}
	}
	return ""
}

func hasESXIFiles(entries []fs.DirEntry) bool {
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if n == "s.v00" || n == "c.gz" || strings.HasPrefix(n, "vmware-") {
			return true
		}
	}
	return false
}

func detectESXIVersion(entries []fs.DirEntry) string {
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if strings.HasPrefix(n, "vmware-esx-") || strings.HasPrefix(n, "vmware-vmvisor-") {
			return strings.TrimPrefix(strings.TrimPrefix(n, "vmware-esx-"), "vmware-vmvisor-")
		}
	}
	return ""
}

func hasWindowsFiles(entries []fs.DirEntry) bool {
	for _, e := range entries {
		n := strings.ToUpper(e.Name())
		if n == "AUTORUN.INF" || n == "SOURCES" && e.IsDir() {
			return true
		}
	}
	return false
}

func detectArch(mountPoint string, entries []fs.DirEntry, distro string) string {
	if distro == "ubuntu" || distro == "debian" {
		if d, err := os.ReadDir(filepath.Join(mountPoint, "casper")); err == nil {
			for _, e := range d {
				if strings.Contains(e.Name(), "amd64") || strings.Contains(e.Name(), "x86_64") {
					return "amd64"
				}
				if strings.Contains(e.Name(), "arm64") || strings.Contains(e.Name(), "aarch64") {
					return "arm64"
				}
			}
		}
	}
	if distro == "windows" {
		for _, e := range entries {
			if strings.ToLower(e.Name()) == "sources" && e.IsDir() {
				if _, err := os.Stat(filepath.Join(mountPoint, "sources", "boot.wim")); err == nil {
					return "amd64"
				}
			}
		}
	}
	return "amd64"
}

// MountISO mounts an ISO and returns the actual accessible mount path.
// On Linux: mountPoint is used directly and returned as-is.
// On Windows: Mount-DiskImage mounts to a virtual drive letter; that path is returned.
func MountISO(src, mountPoint string) (string, error) {
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return "", fmt.Errorf("create mount point: %w", err)
	}
	if runtime.GOOS == "windows" {
		return mountWindowsISO(src)
	}
	if err := mountLinuxISO(src, mountPoint); err != nil {
		return "", err
	}
	return mountPoint, nil
}

func mountLinuxISO(src, mountPoint string) error {
	cmd := exec.Command("mount", "-o", "loop,ro", src, mountPoint)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mountWindowsISO(src string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`$img = Mount-DiskImage -ImagePath "%s" -StorageType ISO -Access ReadOnly -PassThru; ($img | Get-Volume).DriveLetter`, src))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("mount iso on windows: %w", err)
	}
	driveLetter := strings.TrimSpace(string(out))
	if driveLetter == "" {
		return "", fmt.Errorf("mount iso on windows: no drive letter returned")
	}
	return driveLetter + ":\\", nil
}

// UnmountISO dismounts an ISO.
// On Windows: dismounts by the original ISO source path.
// On Linux: unmounts by mount point.
func UnmountISO(mountPoint, isoPath string) error {
	if runtime.GOOS == "windows" {
		return unmountWindowsISO(isoPath)
	}
	return unmountLinuxISO(mountPoint)
}

func unmountLinuxISO(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func unmountWindowsISO(isoPath string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Dismount-DiskImage -ImagePath "%s"`, isoPath))
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ValidateISO(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 0x8001+6)
	if _, err := f.Read(buf); err != nil {
		return fmt.Errorf("cannot read header: %w", err)
	}
	isoMagic := string(buf[1:6])
	if isoMagic != "CD001" {
		magic2 := string(buf[0x8001 : 0x8001+5])
		if magic2 != "CD001" {
			return fmt.Errorf("not a valid ISO 9660 image")
		}
	}
	return nil
}

func ExtractISO(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	if runtime.GOOS == "windows" {
		return extractWindowsISO(src, dest)
	}
	return extractLinuxISO(src, dest)
}

func extractLinuxISO(src, dest string) error {
	cmd := exec.Command("7z", "x", src, fmt.Sprintf("-o%s", dest), "-y")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract iso with 7z: %w", err)
	}
	return nil
}

func extractWindowsISO(src, dest string) error {
	psCmd := fmt.Sprintf(
		`$img = Mount-DiskImage -ImagePath "%s" -StorageType ISO -PassThru; $vol = $img | Get-Volume; $drv = $vol.DriveLetter + ":\"; Copy-Item -Path "$drv*" -Destination "%s" -Recurse -Force; Dismount-DiskImage -ImagePath "%s"`,
		src, dest, src)
	cmd := exec.Command("powershell", "-Command", psCmd)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func FindKernelInitrd(mountPoint string, distro string) (kernel string, initrd string) {
	switch distro {
	case "ubuntu":
		kernel = filepath.Join(mountPoint, "casper", "vmlinuz")
		initrd = filepath.Join(mountPoint, "casper", "initrd")
		if _, err := os.Stat(kernel); err != nil {
			kernel = filepath.Join(mountPoint, "install", "vmlinuz")
			initrd = filepath.Join(mountPoint, "install", "initrd.gz")
		}
	case "debian":
		kernel = filepath.Join(mountPoint, "install", "amd64", "linux")
		initrd = filepath.Join(mountPoint, "install", "amd64", "initrd.gz")
	case "centos", "rocky", "almalinux":
		kernel = filepath.Join(mountPoint, "images", "pxeboot", "vmlinuz")
		initrd = filepath.Join(mountPoint, "images", "pxeboot", "initrd.img")
	case "esxi":
		kernel = filepath.Join(mountPoint, "mboot.c32")
		initrd = ""
	default:
		kernel = filepath.Join(mountPoint, "casper", "vmlinuz")
		initrd = filepath.Join(mountPoint, "casper", "initrd")
	}
	return
}
