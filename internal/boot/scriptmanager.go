package boot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var validScriptID = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func validateScriptID(id string) error {
	if id == "" || len(id) > 256 {
		return fmt.Errorf("invalid script id: empty or too long")
	}
	if !validScriptID.MatchString(id) {
		return fmt.Errorf("invalid script id: contains unsafe characters")
	}
	return nil
}

type ScriptMeta struct {
	ID        string    `json:"id"`
	ProfileID string    `json:"profile_id"`
	EntryIdx  int       `json:"entry_idx"`
	Label     string    `json:"label"`
	Type      string    `json:"type"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScriptVersion struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Checksum  string    `json:"checksum"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ScriptManager struct {
	rootDir string
	mu      sync.RWMutex
}

func NewScriptManager(rootDir string) *ScriptManager {
	if err := os.MkdirAll(filepath.Join(rootDir, ".scripts", "versions"), 0755); err != nil {
		// non-fatal; individual Save calls will create dirs as needed
	}
	return &ScriptManager{rootDir: rootDir}
}

func (sm *ScriptManager) scriptDir(scriptID string) string {
	return filepath.Join(sm.rootDir, ".scripts", scriptID)
}

func (sm *ScriptManager) contentPath(scriptID string) string {
	return filepath.Join(sm.scriptDir(scriptID), "current.ipxe")
}

func (sm *ScriptManager) metaPath(scriptID string) string {
	return filepath.Join(sm.scriptDir(scriptID), "meta.json")
}

func (sm *ScriptManager) versionsDir(scriptID string) string {
	return filepath.Join(sm.scriptDir(scriptID), "versions")
}

func (sm *ScriptManager) versionPath(scriptID, verID string) string {
	return filepath.Join(sm.versionsDir(scriptID), verID+".ipxe")
}

// Save saves script content, creating a new version snapshot.
func (sm *ScriptManager) Save(scriptID, profileID, label, entryType, content, comment string) (*ScriptVersion, error) {
	if err := validateScriptID(scriptID); err != nil {
		return nil, err
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	dir := sm.scriptDir(scriptID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create script dir: %w", err)
	}

	checksum := sha256Hex(content)
	now := time.Now().UTC()
	verID := fmt.Sprintf("%s-%s", now.Format("20060102T150405"), checksum[:8])

	meta := ScriptMeta{
		ID:        scriptID,
		ProfileID: profileID,
		EntryIdx:  0,
		Label:     label,
		Type:      entryType,
		UpdatedAt: now,
	}
	if err := writeJSON(sm.metaPath(scriptID), meta); err != nil {
		return nil, err
	}

	if err := os.WriteFile(sm.contentPath(scriptID), []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write current: %w", err)
	}

	vd := sm.versionsDir(scriptID)
	os.MkdirAll(vd, 0755)

	ver := ScriptVersion{
		ID:        verID,
		Content:   content,
		Checksum:  checksum,
		Comment:   comment,
		CreatedAt: now,
	}
	if err := os.WriteFile(sm.versionPath(scriptID, verID), []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write version: %w", err)
	}

	return &ver, nil
}

// SaveIfChanged saves only if content differs from current.
// Holds the write lock for the entire compare-and-save to prevent races.
func (sm *ScriptManager) SaveIfChanged(scriptID, profileID, label, entryType, content, comment string) (*ScriptVersion, bool, error) {
	if err := validateScriptID(scriptID); err != nil {
		return nil, false, err
	}
	// Use a single write lock to prevent TOCTOU between read and write
	sm.mu.Lock()
	defer sm.mu.Unlock()

	current, err := os.ReadFile(sm.contentPath(scriptID))
	if err == nil && string(current) == content {
		return nil, false, nil
	}
	// Call the inner save (no lock — we already hold it)
	ver, err := sm.saveLocked(scriptID, profileID, label, entryType, content, comment)
	return ver, true, err
}

// saveLocked performs Save while caller holds sm.mu.
func (sm *ScriptManager) saveLocked(scriptID, profileID, label, entryType, content, comment string) (*ScriptVersion, error) {
	dir := sm.scriptDir(scriptID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create script dir: %w", err)
	}

	checksum := sha256Hex(content)
	now := time.Now().UTC()
	verID := fmt.Sprintf("%s-%s", now.Format("20060102T150405"), checksum[:8])

	meta := ScriptMeta{
		ID:        scriptID,
		ProfileID: profileID,
		EntryIdx:  0,
		Label:     label,
		Type:      entryType,
		UpdatedAt: now,
	}
	if err := writeJSON(sm.metaPath(scriptID), meta); err != nil {
		return nil, err
	}
	if err := os.WriteFile(sm.contentPath(scriptID), []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write current: %w", err)
	}
	vd := sm.versionsDir(scriptID)
	os.MkdirAll(vd, 0755)
	ver := ScriptVersion{
		ID:        verID,
		Content:   content,
		Checksum:  checksum,
		Comment:   comment,
		CreatedAt: now,
	}
	if err := os.WriteFile(sm.versionPath(scriptID, verID), []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write version: %w", err)
	}
	return &ver, nil
}

// Load returns current script content.
func (sm *ScriptManager) Load(scriptID string) (string, error) {
	if err := validateScriptID(scriptID); err != nil {
		return "", err
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	data, err := os.ReadFile(sm.contentPath(scriptID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetMeta returns script metadata.
func (sm *ScriptManager) GetMeta(scriptID string) (*ScriptMeta, error) {
	if err := validateScriptID(scriptID); err != nil {
		return nil, err
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var meta ScriptMeta
	data, err := os.ReadFile(sm.metaPath(scriptID))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ListVersions returns all saved versions for a script.
func (sm *ScriptManager) ListVersions(scriptID string) ([]ScriptVersion, error) {
	if err := validateScriptID(scriptID); err != nil {
		return nil, err
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	vd := sm.versionsDir(scriptID)
	entries, err := os.ReadDir(vd)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptVersion{}, nil
		}
		return nil, err
	}

	versions := make([]ScriptVersion, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ipxe") {
			continue
		}
		verID := strings.TrimSuffix(e.Name(), ".ipxe")
		data, err := os.ReadFile(filepath.Join(vd, e.Name()))
		if err != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		versions = append(versions, ScriptVersion{
			ID:        verID,
			Content:   string(data),
			Checksum:  sha256Hex(string(data)),
			CreatedAt: fi.ModTime().UTC(),
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})
	return versions, nil
}

// GetVersion returns a specific version.
func (sm *ScriptManager) GetVersion(scriptID, verID string) (*ScriptVersion, error) {
	if err := validateScriptID(scriptID); err != nil {
		return nil, err
	}
	if verID == "" || len(verID) > 256 || strings.Contains(verID, "/") || strings.Contains(verID, "\\") || strings.Contains(verID, "..") {
		return nil, fmt.Errorf("invalid version id")
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	data, err := os.ReadFile(sm.versionPath(scriptID, verID))
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(sm.versionPath(scriptID, verID))
	if err != nil {
		return nil, err
	}
	return &ScriptVersion{
		ID:        verID,
		Content:   string(data),
		Checksum:  sha256Hex(string(data)),
		CreatedAt: fi.ModTime().UTC(),
	}, nil
}

// Diff returns the diff between a version and current content.
func (sm *ScriptManager) Diff(scriptID, verID string) (string, error) {
	current, err := sm.Load(scriptID)
	if err != nil {
		return "", err
	}
	ver, err := sm.GetVersion(scriptID, verID)
	if err != nil {
		return "", err
	}
	return simpleDiff(ver.Content, current), nil
}

// Rollback restores a previous version as the current, preserving profile metadata.
func (sm *ScriptManager) Rollback(scriptID, verID, comment string) (*ScriptVersion, error) {
	ver, err := sm.GetVersion(scriptID, verID)
	if err != nil {
		return nil, err
	}
	// Preserve profile association from existing meta
	meta, _ := sm.GetMeta(scriptID)
	profileID := ""
	label := scriptID
	entryType := "custom"
	if meta != nil {
		profileID = meta.ProfileID
		label = meta.Label
		entryType = meta.Type
	}
	return sm.Save(scriptID, profileID, label, entryType, ver.Content, comment)
}

// ListScripts returns all known scripts.
func (sm *ScriptManager) ListScripts() ([]ScriptMeta, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.listScriptsLocked()
}

func (sm *ScriptManager) listScriptsLocked() ([]ScriptMeta, error) {
	scriptsDir := filepath.Join(sm.rootDir, ".scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptMeta{}, nil
		}
		return nil, err
	}

	scripts := make([]ScriptMeta, 0)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "versions" {
			continue
		}
		metaPath := filepath.Join(scriptsDir, e.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta ScriptMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		scripts = append(scripts, meta)
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].UpdatedAt.After(scripts[j].UpdatedAt)
	})
	return scripts, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func simpleDiff(oldStr, newStr string) string {
	oldLines := splitLines(oldStr)
	newLines := splitLines(newStr)
	if linesEqual(oldLines, newLines) {
		return ""
	}
	var b strings.Builder
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			b.WriteString("  " + oldLines[i] + "\n")
			i++
			j++
		} else {
			ni, nj := i+1, j+1
			for ni < len(oldLines) && nj < len(newLines) && oldLines[ni] != newLines[nj] {
				ni++
				nj++
			}
			for i < ni {
				b.WriteString("- " + oldLines[i] + "\n")
				i++
			}
			for j < nj {
				b.WriteString("+ " + newLines[j] + "\n")
				j++
			}
		}
	}
	for i < len(oldLines) {
		b.WriteString("- " + oldLines[i] + "\n")
		i++
	}
	for j < len(newLines) {
		b.WriteString("+ " + newLines[j] + "\n")
		j++
	}
	return b.String()
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Trailing newline produces empty last element — trim it for clean diff
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
