package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

type AnswerTemplateHandler struct {
	store store.Interface
}

func (h *AnswerTemplateHandler) Validate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		Error(w, http.StatusBadRequest, "cannot read body")
		return
	}
	content := string(body)

	var parseErr string
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}
	_, err = template.New("answer").Funcs(funcMap).Parse(content)
	if err != nil {
		parseErr = err.Error()
	}

	OK(w, map[string]any{
		"valid":  err == nil,
		"error":  parseErr,
	})
}

func (h *AnswerTemplateHandler) Preview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	tpl, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "template not found")
		return
	}

	var req struct {
		HostName    string `json:"host_name"`
		HostIP      string `json:"host_ip"`
		HostMAC     string `json:"host_mac"`
		HostCIDR    string `json:"host_cidr"`
		Gateway     string `json:"gateway"`
		DNSServers  string `json:"dns_servers"`
		Disk        string `json:"disk"`
		KeyboardLayout string `json:"keyboard_layout"`
		Arch        string `json:"arch"`
		ProductKey    string `json:"product_key"`
		ComputerName  string `json:"computer_name"`
		JoinDomain    string `json:"join_domain"`
		DomainOU      string `json:"domain_ou"`
		AdminPassword string `json:"admin_password"`
		TimeZone      string `json:"time_zone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}

	data := netboot.AnswerData{
		HostName:    req.HostName,
		HostIP:      req.HostIP,
		HostMAC:     req.HostMAC,
		HostCIDR:    req.HostCIDR,
		Gateway:     req.Gateway,
		DNSServers:  req.DNSServers,
		Disk:        req.Disk,
		KeyboardLayout: req.KeyboardLayout,
		Arch:        req.Arch,
		ProductKey:    req.ProductKey,
		ComputerName:  req.ComputerName,
		JoinDomain:    req.JoinDomain,
		DomainOU:      req.DomainOU,
		AdminPassword: req.AdminPassword,
		TimeZone:      req.TimeZone,
	}

	rendered, err := netboot.RenderAnswerTemplate(tpl.Content, data)
	if err != nil {
		Error(w, http.StatusBadRequest, "render failed: "+err.Error())
		return
	}

	OK(w, map[string]any{
		"rendered": rendered,
	})
}

// Presets returns a list of built-in answer templates.
func (h *AnswerTemplateHandler) Presets(w http.ResponseWriter, r *http.Request) {
	typeEntry := r.URL.Query().Get("type")
	if typeEntry == "" {
		presets := make([]map[string]any, 0)
		for t := range builtinPresets {
			presets = append(presets, map[string]any{"type": t, "count": len(builtinPresets[t])})
		}
		OK(w, map[string]any{"presets": presets})
		return
	}

	items, ok := builtinPresets[typeEntry]
	if !ok {
		Error(w, http.StatusNotFound, "no presets for type: "+typeEntry)
		return
	}
	OK(w, map[string]any{"presets": items})
}

var builtinPresets = map[string][]struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Variables   []string `json:"variables"`
}{
	"preseed": {{
		Name:        "Ubuntu Server 22.04+ Autoinstall",
		Description: "Standard Ubuntu Server automated installation using subiquity autoinstall format",
		Variables:   []string{"root_password", "timezone", "ssh_key"},
		Content: `#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: {{.HostName}}
    password: "{{.HostName | upper}}"
    username: ubuntu
  ssh:
    install-server: true
    authorized-keys:
      - "{{.HostName | upper}}"
  locale: en_US.UTF-8
  keyboard:
    layout: us
  network:
    version: 2
    ethernets:
      eth0:
        dhcp4: true
  storage:
    layout:
      name: lvm
  packages:
    - openssh-server
    - qemu-guest-agent
  late-commands:
    - echo "done" > /install-finished`,
	}, {
		Name:        "Debian 12 Preseed",
		Description: "Standard Debian 12 Bookworm preseed configuration",
		Variables:   []string{"root_password", "timezone"},
		Content: `d-i debian-installer/locale string en_US.UTF-8
d-i keyboard-configuration/xkb-keymap select us
d-i netcfg/choose_interface select auto
d-i netcfg/get_hostname string {{.HostName}}
d-i netcfg/get_domain string local
d-i passwd/root-password password {{.HostName | upper}}
d-i passwd/root-password-again password {{.HostName | upper}}
d-i passwd/user-fullname string Ubuntu User
d-i passwd/username string ubuntu
d-i passwd/user-password password {{.HostName | upper}}
d-i passwd/user-password-again password {{.HostName | upper}}
d-i time/zone string UTC
d-i clock-setup/utc boolean true
d-i clock-setup/ntp boolean true
d-i partman-auto/method string regular
d-i partman-auto/choose_recipe select atomic
d-i partman-partitioning/confirm_write_new_label boolean true
d-i partman/choose_partition select finish
d-i partman/confirm boolean true
d-i partman/confirm_nooverwrite boolean true
d-i apt-setup/use_mirror boolean true
d-i pkgsel/include string openssh-server
d-i grub-installer/only_debian boolean true
d-i finish-install/reboot_in_progress note`,
	}, {
		Name:        "Ubuntu 20.04 Legacy Preseed",
		Description: "Legacy debconf preseed for Ubuntu 20.04 and earlier",
		Variables:   []string{"root_password", "timezone"},
		Content: `d-i debian-installer/locale string en_US.UTF-8
d-i keyboard-configuration/xkb-keymap select us
d-i netcfg/choose_interface select auto
d-i netcfg/get_hostname string {{.HostName}}
d-i netcfg/get_domain string local
d-i passwd/root-password password {{.HostName | upper}}
d-i passwd/root-password-again password {{.HostName | upper}}
d-i passwd/user-fullname string Ubuntu User
d-i passwd/username string ubuntu
d-i passwd/user-password password {{.HostName | upper}}
d-i passwd/user-password-again password {{.HostName | upper}}
d-i time/zone string UTC
d-i clock-setup/utc boolean true
d-i partman-auto/method string regular
d-i partman-auto/choose_recipe select atomic
d-i partman-partitioning/confirm_write_new_label boolean true
d-i partman/confirm boolean true
d-i grub-installer/only_debian boolean true
d-i finish-install/reboot_in_progress note`,
	}},
	"kickstart": {{
		Name:        "RHEL/CentOS/Rocky/Alma 8+ Kickstart",
		Description: "Standard kickstart for RHEL-family distributions 8+",
		Variables:   []string{"root_password", "timezone", "ssh_key"},
		Content: `#version=RHEL8
lang en_US.UTF-8
keyboard us
timezone {{.HostName | upper}} --isUtc
rootpw --iscrypted {{.HostName | upper}}
user --name=admin --password={{.HostName | upper}} --iscrypted --groups=wheel
text
skipx
cdrom
url --url="http://{{.HostIP}}/install"
network --bootproto=dhcp --hostname={{.HostName}}
services --enabled=sshd,NetworkManager,chronyd
firewall --enabled --service=ssh
selinux --enforcing
bootloader --location=mbr --append="crashkernel=auto quiet"
zerombr
clearpart --all --initlabel
autopart
%packages
@^minimal-environment
kexec-tools
%end
%addon com_redhat_kdump --enable --reserve-mb='auto'
%end
reboot`,
	}, {
		Name:        "Rocky/Alma 9 Minimal Kickstart",
		Description: "Minimal kickstart for Rocky/Alma Linux 9",
		Variables:   []string{"root_password"},
		Content: `#version=RHEL9
lang en_US.UTF-8
keyboard us
timezone UTC
rootpw {{.HostName | upper}}
user --name=admin --password={{.HostName | upper}} --iscrypted
text
skipx
url --url="http://{{.HostIP}}/install"
network --bootproto=dhcp --hostname={{.HostName}}
services --enabled=sshd
firewall --enabled --service=ssh
selinux --enforcing
bootloader --location=mbr
zerombr
clearpart --all --initlabel
autopart --type=lvm
%packages
@core
%end
reboot`,
	}},
	"subiquity": {{
		Name:        "Ubuntu Server 24.04 Autoinstall (YAML)",
		Description: "Subiquity autoinstall format for Ubuntu Server 24.04 LTS",
		Variables:   []string{"ssh_key", "timezone"},
		Content: `#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: {{.HostName}}
    password: "$6$rounds=4096$salt$hashed"
    username: ubuntu
  ssh:
    install-server: true
    authorized-keys:
      - "{{.HostName | upper}}"
  locale: en_US.UTF-8
  keyboard:
    layout: us
  network:
    version: 2
    ethernets:
      eth0:
        dhcp4: true
  storage:
    layout:
      name: lvm
  packages:
    - openssh-server
    - curl
    - wget
  late-commands:
    - echo 'PXE installed' > /etc/motd`,
	}},
	"autoyast": {{
		Name:        "openSUSE/SLES AutoYaST",
		Description: "AutoYaST XML configuration for SUSE Linux Enterprise and openSUSE",
		Variables:   []string{"root_password", "timezone"},
		Content: `<?xml version="1.0"?>
<!DOCTYPE profile>
<profile xmlns="http://www.suse.com/1.0/yast2ns">
  <general>
    <mode>
      <confirm config:type="boolean">false</confirm>
    </mode>
  </general>
  <networking>
    <dhcp_options>
      <dhcp_hostname config:type="boolean">true</dhcp_hostname>
    </dhcp_options>
    <interfaces config:type="list">
      <interface>
        <bootproto>dhcp</bootproto>
        <name>eth0</name>
      </interface>
    </interfaces>
    <hostname>{{.HostName}}</hostname>
  </networking>
  <users>
    <user>
      <username>root</username>
      <user_password>{{.HostName | upper}}</user_password>
      <encrypted config:type="boolean">false</encrypted>
    </user>
  </users>
  <software>
    <products config:type="list">
      <product>SLES</product>
    </products>
  </software>
  <partitioning config:type="list">
    <drive>
      <device>/dev/sda</device>
      <use>all</use>
    </drive>
  </partitioning>
</profile>`,
	}},
	"autounattend": {{
		Name:        "Windows Server 2022 Autounattend",
		Description: "Autounattend XML for Windows Server 2022/2019 automated installation",
		Variables:   []string{"product_key", "admin_password", "computer_name"},
		Content: `<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend">
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE">
      <InputLocale>en-US</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
  </settings>
  <settings pass="offlineServicing">
    <component name="Microsoft-Windows-Shell-Setup">
      <ComputerName>{{.HostName}}</ComputerName>
    </component>
  </settings>
  <settings pass="specialize">
    <component name="Microsoft-Windows-Shell-Setup">
      <ComputerName>{{.HostName}}</ComputerName>
      <ProductKey>{{.HostName | upper}}</ProductKey>
    </component>
  </settings>
  <settings pass="oobeSystem">
    <component name="Microsoft-Windows-Shell-Setup">
      <UserAccounts>
        <AdministratorPassword>
          <Value>{{.HostName | upper}}</Value>
          <PlainText>true</PlainText>
        </AdministratorPassword>
      </UserAccounts>
      <AutoLogon>
        <Enabled>true</Enabled>
        <Username>Administrator</Username>
        <Password>
          <Value>{{.HostName | upper}}</Value>
          <PlainText>true</PlainText>
        </Password>
      </AutoLogon>
      <OOBE>
        <HideEULAPage>true</HideEULAPage>
        <SkipMachineOOBE>true</SkipMachineOOBE>
        <SkipUserOOBE>true</SkipUserOOBE>
      </OOBE>
    </component>
  </settings>
</unattend>`,
	}},
}

func (h *AnswerTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListAnswerTemplates(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"templates": templates})
}

func (h *AnswerTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var t models.AnswerTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	t.CurrentVersion = 1
	if err := h.store.CreateAnswerTemplate(r.Context(), &t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Create initial version snapshot
	if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
		TemplateID:  t.ID,
		Version:     1,
		Content:     t.Content,
		Description: "初始版本",
	}); err != nil {
		// Rollback template creation if version snapshot fails
		h.store.DeleteAnswerTemplate(r.Context(), t.ID)
		Error(w, http.StatusInternalServerError, "版本快照创建失败")
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "answer_template", fmt.Sprintf("%d", t.ID), remoteIP(r), "新建应答模板: "+t.Name)
	Created(w, t)
}

func (h *AnswerTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	t, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}
	OK(w, t)
}

func (h *AnswerTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var t models.AnswerTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	t.ID = uint(id)

	// Fetch current template to detect content change
	old, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}

	if old.Content != t.Content {
		// Bump version and create snapshot
		t.CurrentVersion = old.CurrentVersion + 1
		if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
			TemplateID:  t.ID,
			Version:     t.CurrentVersion,
			Content:     t.Content,
			Description: t.Description,
		}); err != nil {
			Error(w, http.StatusInternalServerError, "版本快照创建失败")
			return
		}
	} else {
		t.CurrentVersion = old.CurrentVersion
	}

	if err := h.store.UpdateAnswerTemplate(r.Context(), &t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	if old.Name != t.Name {
		changes = append(changes, fmt.Sprintf("名称: %s→%s", old.Name, t.Name))
	}
	if old.Type != t.Type {
		changes = append(changes, fmt.Sprintf("类型: %s→%s", old.Type, t.Type))
	}
	if old.Description != t.Description {
		changes = append(changes, fmt.Sprintf("描述: %s→%s", old.Description, t.Description))
	}
	if old.Content != t.Content {
		changes = append(changes, "内容已变更")
	}
	detail := "更新应答模板: " + t.Name
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "answer_template", fmt.Sprintf("%d", id), remoteIP(r), detail)
	OK(w, t)
}

func (h *AnswerTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	// 保存旧值用于审计
	oldT, _ := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err := h.store.DeleteAnswerTemplate(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := "删除应答模板"
	if oldT != nil && oldT.Name != "" {
		detail = fmt.Sprintf("删除应答模板 %s (%s)", oldT.Name, oldT.Type)
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "answer_template", fmt.Sprintf("%d", id), remoteIP(r), detail)
	_ = h.store.DeleteAnswerTemplateVersions(r.Context(), uint(id))
	w.WriteHeader(http.StatusNoContent)
}

// ListVersions returns all versions for a template.
func (h *AnswerTemplateHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	versions, err := h.store.ListAnswerTemplateVersions(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"versions": versions})
}

// GetVersion returns a specific version's content.
func (h *AnswerTemplateHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	ver, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本号")
		return
	}
	v, err := h.store.GetAnswerTemplateVersion(r.Context(), uint(id), ver)
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}
	OK(w, v)
}

// Rollback rolls back to a specific version.
func (h *AnswerTemplateHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	ver, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本号")
		return
	}

	// Fetch the old version
	v, err := h.store.GetAnswerTemplateVersion(r.Context(), uint(id), ver)
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}

	// Fetch current template
	t, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}

	// Snapshot current content before rollback
	newVer := t.CurrentVersion + 1
	if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
		TemplateID:  t.ID,
		Version:     newVer,
		Content:     t.Content,
		Description: "回滚到版本 " + strconv.Itoa(ver),
	}); err != nil {
		Error(w, http.StatusInternalServerError, "版本快照创建失败")
		return
	}

	// Restore old version content
	t.Content = v.Content
	t.CurrentVersion = newVer
	if err := h.store.UpdateAnswerTemplate(r.Context(), t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, t)
}
