package netboot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SyncFromUpstream pulls the latest from netboot.xyz fork and converts to catalog YAML
// If repoDir does not exist, it will be cloned from repoURL.
// ansibleVersion holds a single version entry from netboot.xyz Ansible YAML
type ansibleVersion struct {
	Codename string `yaml:"code_name"`
	Name     string `yaml:"name"`
	Arch     string `yaml:"arch"`
}

// ansibleVersions handles both list and map-of-lists formats:
//   versions:           vs:    versions:
//     - code_name: x            stable:
//       name: X                   - code_name: x
//       arch: amd64                 name: X
//                                  arch: amd64
type ansibleVersions []ansibleVersion

func (v *ansibleVersions) UnmarshalYAML(value *yaml.Node) error {
	var slice []ansibleVersion
	if err := value.Decode(&slice); err == nil {
		*v = slice
		return nil
	}
	var grouped map[string][]ansibleVersion
	if err := value.Decode(&grouped); err != nil {
		return err
	}
	for _, list := range grouped {
		*v = append(*v, list...)
	}
	return nil
}

func SyncFromUpstream(repoDir, catalogDir, repoURL string) error {
	if repoDir == "" {
		return fmt.Errorf("repo directory is required")
	}
	if catalogDir == "" {
		return fmt.Errorf("catalog directory is required")
	}

	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return fmt.Errorf("create catalog dir: %w", err)
	}

	// Auto-clone if repo doesn't exist
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		if repoURL == "" {
			return fmt.Errorf("repo %s does not exist and no repo URL configured", repoDir)
		}
		fmt.Printf("  正在克隆 netboot.xyz 仓库 (%s)...\n", repoURL)
		parent := filepath.Dir(repoDir)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}
		cmd := exec.Command("git", "clone", repoURL, repoDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	} else {
		// Try to pull latest
		if err := gitPull(repoDir); err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
	}

	// Parse Ansible defaults
	defaultsPath := filepath.Join(repoDir, "roles", "netbootxyz", "defaults", "main.yml")
	endpointsPath := filepath.Join(repoDir, "endpoints.yml")

	// Check if defaults file exists
	if _, err := os.Stat(defaultsPath); os.IsNotExist(err) {
		return fmt.Errorf("defaults file not found at %s: %w\nRun: git clone https://github.com/netbootxyz/netboot.xyz.git %s",
			defaultsPath, err, repoDir)
	}

	// Read and parse Ansible YAML
	data, err := os.ReadFile(defaultsPath)
	if err != nil {
		return fmt.Errorf("read defaults file: %w", err)
	}

	var ad struct {
		Releases map[string]struct {
			Name          string          `yaml:"name"`
			Enabled       bool            `yaml:"enabled"`
			Menu          string          `yaml:"menu"`
			BaseDir       string          `yaml:"base_dir"`
			Mirror        string          `yaml:"mirror"`
			ArchiveMirror string          `yaml:"archive_mirror"`
			Versions      ansibleVersions `yaml:"versions"`
		} `yaml:"releases"`
	}

	if err := yaml.Unmarshal(data, &ad); err != nil {
		return fmt.Errorf("parse Ansible defaults: %w", err)
	}

	// Try to parse endpoints for URL construction
	endpoints := loadEndpoints(endpointsPath)

	count := 0
	for releaseKey, release := range ad.Releases {
		if !release.Enabled {
			continue
		}

		menuGroup := mapAnsibleMenu(release.Menu)
		distro := &Distro{
			Name:          release.Name,
			Enabled:       true,
			MenuGroup:     menuGroup,
			Mirror:        release.Mirror,
			ArchiveMirror: release.ArchiveMirror,
			LocalBase:     fmt.Sprintf("netboot/%s", releaseKey),
		}

		for _, av := range release.Versions {
			if av.Codename == "" {
				continue
			}
			arch := av.Arch
			if arch == "" {
				arch = "amd64"
			}
			v := &Version{
				Codename:    av.Codename,
				Name:        av.Name,
				Arch:        arch,
				Enabled:     true,
				InstallType: inferInstallType(release.Menu),
			}

			// Construct remote URLs from endpoints or mirror
			if ep, ok := endpoints[releaseKey]; ok {
				v.Remote = buildURLFromEndpoint(ep, av.Codename, arch, release.Mirror)
			} else if release.Mirror != "" {
				v.Remote = constructRemoteURL(release.Mirror, release.BaseDir, av.Codename, arch)
			}

			distro.Versions = append(distro.Versions, v)
		}

		if len(distro.Versions) > 0 {
			outPath := filepath.Join(catalogDir, fmt.Sprintf("%s.yaml", releaseKey))
			if err := SaveDistro(outPath, distro); err != nil {
				return fmt.Errorf("save %s: %w", releaseKey, err)
			}
			fmt.Printf("  ✓ %s (%d versions)\n", release.Name, len(distro.Versions))
			count++
		}
	}

	if count == 0 {
		fmt.Println("  没有找到启用的发行版")
	} else {
		fmt.Printf("  共 %d 个发行版\n", count)
	}

	return nil
}

type endpointConfig struct {
	KernelTemplate string `yaml:"kernel"`
	InitrdTemplate string `yaml:"initrd"`
}

func loadEndpoints(path string) map[string]endpointConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var eps struct {
		Endpoints map[string]endpointConfig `yaml:"endpoints"`
	}
	if err := yaml.Unmarshal(data, &eps); err != nil {
		return nil
	}
	return eps.Endpoints
}

func buildURLFromEndpoint(ep endpointConfig, codename, arch, mirror string) *FileRef {
	if ep.KernelTemplate == "" {
		return nil
	}
	kernel := strings.ReplaceAll(ep.KernelTemplate, "{version}", codename)
	kernel = strings.ReplaceAll(kernel, "{arch}", arch)
	kernel = strings.ReplaceAll(kernel, "{mirror}", mirror)

	initrd := ep.InitrdTemplate
	if initrd == "" {
		// Derive initrd from kernel path
		initrd = strings.ReplaceAll(ep.KernelTemplate, "{version}", codename)
		initrd = strings.ReplaceAll(initrd, "{arch}", arch)
		initrd = strings.ReplaceAll(initrd, "{mirror}", mirror)
		// Replace common kernel file names with initrd
		initrd = strings.Replace(initrd, "linux", "initrd.gz", 1)
		initrd = strings.Replace(initrd, "vmlinuz", "initrd.img", 1)
	} else {
		initrd = strings.ReplaceAll(initrd, "{version}", codename)
		initrd = strings.ReplaceAll(initrd, "{arch}", arch)
		initrd = strings.ReplaceAll(initrd, "{mirror}", mirror)
	}

	if !strings.HasPrefix(kernel, "http") && mirror != "" {
		kernel = strings.TrimRight(mirror, "/") + "/" + strings.TrimLeft(kernel, "/")
	}
	if !strings.HasPrefix(initrd, "http") && mirror != "" {
		initrd = strings.TrimRight(mirror, "/") + "/" + strings.TrimLeft(initrd, "/")
	}

	return &FileRef{Kernel: kernel, Initrd: initrd}
}

func gitPull(repoDir string) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}
	// Check if the current branch has a remote tracking branch configured
	hasRemote := false
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = repoDir
	if err := cmd.Run(); err == nil {
		hasRemote = true
	}
	if !hasRemote {
		return nil // no upstream configured, skip pull
	}
	cmd = exec.Command("git", "pull", "--ff-only")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mapAnsibleMenu(menu string) string {
	mapping := map[string]string{
		"linux":   GroupLinux,
		"bsd":     GroupBSD,
		"live":    GroupLive,
		"unix":    GroupUnix,
		"dos":     GroupDOS,
		"windows": GroupWindows,
	}
	if g, ok := mapping[menu]; ok {
		return g
	}
	return GroupTools
}

func inferInstallType(menu string) InstallType {
	switch menu {
	case "bsd":
		return InstallBSD
	case "live":
		return InstallLive
	default:
		return InstallLegacy
	}
}

func constructRemoteURL(mirror, baseDir, codename, arch string) *FileRef {
	if mirror == "" {
		return nil
	}
	base := strings.TrimRight(mirror, "/")
	dir := strings.Trim(baseDir, "/")
	return &FileRef{
		Kernel: fmt.Sprintf("%s/%s/%s/%s/linux", base, dir, codename, arch),
		Initrd: fmt.Sprintf("%s/%s/%s/%s/initrd.gz", base, dir, codename, arch),
	}
}
