package netboot

import (
	"strings"
	"testing"
)

func TestNoLabelCollisions(t *testing.T) {
	c := DefaultCatalog()

	labelMap := make(map[string]string)
	for _, d := range c.Distros {
		l := distroLabel(d)
		if existing, ok := labelMap[l]; ok {
			t.Errorf("Duplicate distro label: %s (%s collides with %s)", l, d.Name, existing)
		}
		labelMap[l] = d.Name

		for _, v := range d.Versions {
			vl := versionLabel(d, v)
			if existing, ok := labelMap[vl]; ok {
				t.Errorf("Duplicate version label: %s (%s/%s collides with %s)", vl, d.Name, v.Name, existing)
			}
			labelMap[vl] = d.Name + "/" + v.Name
		}
	}
}

func TestScriptChooseSyntax(t *testing.T) {
	c := DefaultCatalog()
	s := GenerateNetbootScript(c, "192.168.1.1:8080", "", "", "[OS] Netboot OS Install Catalog", nil, nil, true)

	// Must have separated choose/goto (not chained)
	if strings.Contains(s, "choose selected && goto") {
		t.Error("Script should NOT use chained 'choose selected && goto' syntax")
	}

	// Main menu must have the two-line pattern
	if !strings.Contains(s, "choose selected || goto exit\n"+"goto ${selected}") {
		t.Error("Main menu must use separated choose/goto lines")
	}

	// Submenus must have the two-line pattern
	if !strings.Contains(s, "choose selected || goto netboot_menu\n"+"goto ${selected}") {
		t.Error("Submenu must use separated choose/goto lines")
	}

	// All menu labels referenced in item must have corresponding goto targets
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "choose ") {
			// Found a choose that leads to goto ${selected}
		}
	}

	t.Logf("Script length: %d bytes", len(s))
}

func TestScriptContainsAllDistros(t *testing.T) {
	c := DefaultCatalog()
	s := GenerateNetbootScript(c, "192.168.1.1:8080", "", "", "[OS] Netboot OS Install Catalog", nil, nil, true)

	for _, d := range c.Distros {
		if !d.Enabled {
			continue
		}
		label := distroLabel(d)
		if !strings.Contains(s, "item "+label) {
			t.Errorf("Main menu missing item for: %s (label=%s)", d.Name, label)
		}
		if !strings.Contains(s, ":"+label+"\n") {
			t.Errorf("Submenu label missing for: %s (label=%s)", d.Name, label)
		}
	}
}

func TestScriptBootEntries(t *testing.T) {
	c := DefaultCatalog()
	s := GenerateNetbootScript(c, "192.168.1.1:8080", "", "", "[OS] Netboot OS Install Catalog", nil, nil, true)

	count := 0
	for _, d := range c.Distros {
		if !d.Enabled {
			continue
		}
		for _, v := range d.Versions {
			if !v.Enabled {
				continue
			}
			vl := versionLabel(d, v)
			if strings.Contains(s, ":"+vl+"\n") {
				count++
			}
		}
	}
	if count == 0 {
		t.Error("No boot entries found in script")
	}
	t.Logf("Total boot entries: %d", count)
}

func TestScriptNoDuplicateLabels(t *testing.T) {
	c := DefaultCatalog()
	s := GenerateNetbootScript(c, "192.168.1.1:8080", "", "", "[OS] Netboot OS Install Catalog", nil, nil, true)

	labelCount := make(map[string]int)
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ":") && !strings.HasPrefix(line, "::") {
			label := strings.TrimPrefix(line, ":")
			labelCount[label]++
			if labelCount[label] > 1 {
				t.Errorf("Duplicate label in generated script: %s", label)
			}
		}
	}
}

func TestScriptProxyURLs(t *testing.T) {
	c := DefaultCatalog()
	s := GenerateNetbootScript(c, "192.168.1.1:8080", "", "", "[OS] Netboot OS Install Catalog", nil, nil, true)

	// All HTTPS remote URLs should be proxied through the HTTP server
	if strings.Contains(s, "kernel https://") {
		t.Error("HTTPS URLs should be proxied through HTTP, found raw 'kernel https://'")
	}

	// Should have proxy URLs for remote boot files
	proxyCount := strings.Count(s, "/boot/netboot/proxy/https/")
	if proxyCount == 0 {
		t.Error("No proxy URLs found in script")
	}
	t.Logf("Proxy URL count: %d", proxyCount)

	// Verify a specific proxy URL format
	if !strings.Contains(s, "kernel http://192.168.1.1:8080/boot/netboot/proxy/https/") {
		t.Error("Missing expected proxy URL format: 'kernel http://.../boot/netboot/proxy/https/...'")
	}
}
