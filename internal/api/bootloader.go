package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/pxelab/pxelab/internal/boot"
)

type BootloaderHandler struct {
	bootFS *boot.BootFileServer
}

func NewBootloaderHandler(bootFS *boot.BootFileServer) *BootloaderHandler {
	return &BootloaderHandler{bootFS: bootFS}
}

var knownBootFiles = []struct {
	Name        string
	Description string
	Required    bool
}{
	{"ipxe.efi", "iPXE x86_64 EFI", true},
	{"ipxe32.efi", "iPXE IA32 EFI", false},
	{"ipxe.pxe", "iPXE BIOS PXE", true},
	{"ipxe-snponly.efi", "iPXE SnpOnly EFI", false},
	{"ipxe-arm64.efi", "iPXE ARM64 EFI", false},
	{"undionly.kpxe", "iPXE UNDI BIOS", false},
	{"pxelinux.0", "PXELINUX BIOS", false},
	{"pxelinux.bios", "PXELINUX BIOS (alt)", false},
	{"pxelinux.efi", "PXELINUX EFI", false},
	{"grubx64.efi", "GRUB2 x86_64 EFI", false},
	{"grubaa64.efi", "GRUB2 ARM64 EFI", false},
	{"memdisk", "MEMDISK", false},
	{"menu.c32", "VESAMENU", false},
}

type BootFileInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Present     bool   `json:"present"`
	Size        int64  `json:"size,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	ModTime     string `json:"mod_time,omitempty"`
	Error       string `json:"error,omitempty"`
}

type BootloaderCheckResult struct {
	RootDir string         `json:"root_dir"`
	Total   int            `json:"total"`
	Present int            `json:"present"`
	Missing int            `json:"missing"`
	AllOK   bool           `json:"all_ok"`
	Files   []BootFileInfo `json:"files"`
}

// GET /api/v1/bootloader/check
func (h *BootloaderHandler) Check(w http.ResponseWriter, r *http.Request) {
	rootDir := h.bootFS.Root()
	files := make([]BootFileInfo, 0, len(knownBootFiles))
	var present, missing int

	for _, kf := range knownBootFiles {
		info := BootFileInfo{
			Name:        kf.Name,
			Description: kf.Description,
			Required:    kf.Required,
		}
		fullPath := filepath.Join(rootDir, kf.Name)
		fi, err := os.Stat(fullPath)
		if err != nil {
			missing++
		} else {
			info.Present = true
			info.Size = fi.Size()
			info.ModTime = fi.ModTime().UTC().Format("2006-01-02T15:04:05Z")
			if sum, err := computeSHA256(fullPath); err == nil {
				info.Checksum = sum
			}
			present++
		}
		files = append(files, info)
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].Present != files[j].Present {
			return files[i].Present
		}
		return files[i].Name < files[j].Name
	})

	OK(w, BootloaderCheckResult{
		RootDir: rootDir,
		Total:   len(files),
		Present: present,
		Missing: missing,
		AllOK:   missing == 0,
		Files:   files,
	})
}

// GET /api/v1/bootloader/files
func (h *BootloaderHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.bootFS.List(".")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	type entry struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
		IsDir   bool   `json:"is_dir"`
	}
	result := make([]entry, 0, len(entries))
	for _, e := range entries {
		result = append(result, entry{
			Name:    e.Name(),
			Size:    e.Size(),
			ModTime: e.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
			IsDir:   e.IsDir(),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	OK(w, result)
}

// POST /api/v1/bootloader/check-file
func (h *BootloaderHandler) CheckFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" {
		Error(w, http.StatusBadRequest, "invalid file name")
		return
	}
	fullPath, err := h.bootFS.ResolvePath(req.Name)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid file name: "+err.Error())
		return
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		Error(w, http.StatusNotFound, "file not found")
		return
	}

	info := BootFileInfo{
		Name:    req.Name,
		Present: true,
		Size:    fi.Size(),
		ModTime: fi.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if sum, err := computeSHA256(fullPath); err == nil {
		info.Checksum = sum
	}
	OK(w, info)
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
