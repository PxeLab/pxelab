package api

import (
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/ipmi"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/servicemanager"
	"github.com/pxelab/pxelab/internal/session"
	"github.com/pxelab/pxelab/internal/store"
)

// ServiceController 服务生命周期管理接口
type ServiceController interface {
	List() []servicemanager.ServiceInfo
	Get(name string) (servicemanager.ServiceInfo, bool)
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	BatchStart(names []string) servicemanager.BatchResult
	BatchStop(names []string) servicemanager.BatchResult
	BatchRestart(names []string) servicemanager.BatchResult
	SetAutoStart(name string, enabled bool) error
}

type Handler struct {
	Host            *HostHandler
	Profile         *ProfileHandler
	Event           *EventHandler
	File            *FileHandler
	WOL             *WOLHandler
	IPMI            *IPMIHandler
	Lease           *LeaseHandler
	Settings        *SettingsHandler
	Logs            *LogStreamHandler
	Netboot         *NetbootHandler
	NetbootOverlay  *NetbootOverlayHandler
	AnswerTemplate  *AnswerTemplateHandler
	InstallTask     *InstallTaskHandler
	Service         *ServiceHandler
	Auth            *AuthHandler
	Access          *AccessHandler
	DNSRecord       *DNSRecordHandler
	BMC             *BMCHandler
	DHCPReservation *DHCPReservationHandler
	Network         *NetworkHandler
	OSImage         *OSImageHandler
	Bootloader      *BootloaderHandler
	AuditLog        *AuditLogHandler
	svcController   ServiceController
	sessions        *session.Store
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer, reloader SubnetReloader, netbootMgr *netboot.Manager, svcController ServiceController, sessions *session.Store, setNFSMountPoints func(mps []config.NFSMountPoint), getNFSConnections func() map[string]NFSConnectionInfo, isServiceRunning func(name string) bool) *Handler {
	h := &Handler{
		Host:            &HostHandler{store: st, config: cfg},
		Profile:         &ProfileHandler{store: st, netbootMgr: netbootMgr},
		Event:           NewEventHandler(st, bus),
		File:            &FileHandler{bootFS: bootFS},
		WOL:             &WOLHandler{store: st, config: cfg, eventBus: bus},
		IPMI:            &IPMIHandler{store: st, ipmiClient: ipmi.NewClient()},
		Lease:           &LeaseHandler{store: st, config: cfg},
		Settings:        NewSettingsHandler(cfg, st, reloader, setNFSMountPoints, getNFSConnections, isServiceRunning),
		Logs:            NewLogStreamHandler(bus, logDir(cfg)),
		Netboot:         NewNetbootHandler(netbootMgr),
		NetbootOverlay:  &NetbootOverlayHandler{store: st},
		AnswerTemplate:  &AnswerTemplateHandler{store: st},
		InstallTask:     &InstallTaskHandler{store: st},
		Service:         NewServiceHandler(svcController, cfg, st, func() error { return saveConfig(configPath(cfg), cfg) }),
		Auth:            NewAuthHandler(cfg, sessions),
		Access:          &AccessHandler{store: st},
		DNSRecord:       &DNSRecordHandler{store: st, localDomain: cfg.DNS.LocalDomain},
		BMC:             NewBMCHandler(st),
		DHCPReservation: &DHCPReservationHandler{store: st},
		Network:         &NetworkHandler{},
		OSImage:         NewOSImageHandler(st, cfg),
		Bootloader:      NewBootloaderHandler(bootFS),
		AuditLog:        NewAuditLogHandler(st),
		svcController:   svcController,
		sessions:        sessions,
	}
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", h.Status)
		r.Get("/metrics", h.Metrics)

		// Auth routes (public — login, session check)
		r.Post("/auth/login", h.Auth.Login)
		r.Post("/auth/logout", h.Auth.Logout)
		r.Get("/auth/session", h.Auth.Session)

		r.Get("/events", h.Event.List)
		r.Get("/events/stream", h.Event.Stream)
		r.Get("/logs/stream", h.Logs.Stream)
		r.Get("/logs/files", h.Logs.ListLogFiles)
		r.Get("/logs/disk-usage", h.Logs.DiskUsage)
		r.Post("/logs/cleanup", h.Logs.CleanupLogs)
		r.Get("/audit-logs", h.AuditLog.List)

		r.Get("/hosts", h.Host.List)
		r.Post("/hosts", h.Host.Create)
		r.Get("/hosts/{id}", h.Host.Get)
		r.Put("/hosts/{id}", h.Host.Update)
		r.Delete("/hosts/{id}", h.Host.Delete)
		r.Post("/hosts/{id}/wake", h.WOL.Wake)
		r.Post("/hosts/{id}/power", h.IPMI.PowerAction)
		r.Get("/hosts/{id}/boot-config", h.Host.PreviewBootConfig)

		r.Get("/profiles", h.Profile.List)
		r.Post("/profiles", h.Profile.Create)
		r.Get("/profiles/{id}", h.Profile.Get)
		r.Put("/profiles/{id}", h.Profile.Update)
		r.Delete("/profiles/{id}", h.Profile.Delete)
		r.Post("/profiles/from-netboot", h.Profile.CreateFromNetboot)
		r.Get("/profiles/{profileId}/script-versions", h.Profile.ListScriptVersions)
		r.Get("/profiles/{profileId}/script-versions/{verId}", h.Profile.GetScriptVersion)
		r.Get("/profiles/{profileId}/script-diff/{verId}", h.Profile.DiffScriptVersion)
		r.Post("/profiles/{profileId}/script-rollback/{verId}", h.Profile.RollbackScriptVersion)

		r.Get("/files", h.File.List)
		r.Get("/files/root", h.File.GetRootDir)
		r.Post("/files/upload", h.File.Upload)
		r.Delete("/files", h.File.Delete)

		r.Get("/leases", h.Lease.List)
		r.Get("/leases/stats", h.Lease.Stats)
		r.Delete("/leases/{mac}", h.Lease.Delete)
		r.Post("/leases/batch-delete", h.Lease.BatchDelete)
		r.Post("/leases/prune", h.Lease.Prune)

		// Settings sub-endpoints (individual service configs)
		r.Get("/settings/general", h.Settings.GetGeneral)
		r.Put("/settings/general", h.Settings.UpdateGeneral)
		r.Get("/settings/interfaces", h.Settings.GetInterfaces)
		r.Put("/settings/interfaces", h.Settings.UpdateInterfaces)
		r.Get("/services/tftp", h.Settings.GetTFTP)
		r.Put("/services/tftp", h.Settings.UpdateTFTP)
		r.Get("/services/archmap", h.Settings.GetArchMap)
		r.Put("/services/archmap", h.Settings.UpdateArchMap)
		r.Get("/services/archmap/defaults", h.Settings.GetArchMapDefaults)
		r.Get("/services/dhcp", h.Settings.GetDHCP)
		r.Put("/services/dhcp", h.Settings.UpdateDHCP)
		r.Get("/services/dns", h.Settings.GetDNS)
		r.Put("/services/dns", h.Settings.UpdateDNS)
		r.Get("/services/nfs", h.Settings.GetNFS)
		r.Put("/services/nfs", h.Settings.UpdateNFS)
		r.Post("/services/nfs/validate-path", h.Settings.ValidateNFSPath)
		r.Get("/services/nfs/browse-path", h.Settings.BrowseNFSPath)
		r.Get("/services/ipxe-script", h.Settings.GetIPXEScript)
		r.Put("/services/ipxe-script", h.Settings.UpdateIPXEScript)
		r.Get("/settings/netboot", h.Settings.GetNetboot)
		r.Put("/settings/netboot", h.Settings.UpdateNetboot)
		r.Get("/settings/logging", h.Settings.GetLoggingSettings)
		r.Put("/settings/logging", h.Settings.UpdateLoggingSettings)
		r.Get("/netboot/cache-stats", h.Settings.GetCacheStats)
		// Keep monolithic endpoint for backward compat
		r.Get("/settings", h.Settings.Get)
		r.Put("/settings", h.Settings.Update)

		r.Get("/interfaces", h.ListInterfaces)

		r.Get("/services", h.Service.ListServices)
		r.Post("/services/{name}/start", h.Service.StartService)
		r.Post("/services/{name}/stop", h.Service.StopService)
		r.Post("/services/{name}/restart", h.Service.RestartService)
		r.Post("/services/batch/{action}", h.Service.BatchOperation)
		r.Put("/services/{name}/auto-start", h.Service.UpdateAutoStart)

		r.Get("/netboot/catalog", h.Netboot.GetCatalog)
		r.Get("/netboot/catalog/{distro}", h.Netboot.GetDistro)
		r.Get("/netboot/groups", h.Netboot.GetGroups)
		r.Get("/netboot/check-files", h.Netboot.CheckFiles)

		// Netboot overlays
		r.Get("/netboot/overlays", h.NetbootOverlay.List)
		r.Get("/netboot/overlays/{distro}", h.NetbootOverlay.Get)
		r.Put("/netboot/overlays/{distro}", h.NetbootOverlay.Upsert)
		r.Delete("/netboot/overlays/{distro}", h.NetbootOverlay.Delete)

		// Answer templates
		r.Get("/netboot/answer-templates", h.AnswerTemplate.List)
		r.Post("/netboot/answer-templates", h.AnswerTemplate.Create)
		r.Get("/netboot/answer-templates/{id}", h.AnswerTemplate.Get)
		r.Put("/netboot/answer-templates/{id}", h.AnswerTemplate.Update)
		r.Delete("/netboot/answer-templates/{id}", h.AnswerTemplate.Delete)
		r.Get("/netboot/answer-templates/presets", h.AnswerTemplate.Presets)
		r.Post("/netboot/answer-templates/validate", h.AnswerTemplate.Validate)
		r.Get("/netboot/answer-templates/{id}/versions", h.AnswerTemplate.ListVersions)
		r.Get("/netboot/answer-templates/{id}/versions/{version}", h.AnswerTemplate.GetVersion)
		r.Post("/netboot/answer-templates/{id}/preview", h.AnswerTemplate.Preview)
		r.Post("/netboot/answer-templates/{id}/rollback/{version}", h.AnswerTemplate.Rollback)
		r.Post("/netboot/answer-templates/{id}/validate", h.AnswerTemplate.Validate)

		// Install tasks
		r.Get("/netboot/tasks", h.InstallTask.List)
		r.Post("/netboot/tasks", h.InstallTask.Create)
		r.Get("/netboot/tasks/{id}", h.InstallTask.Get)
		r.Put("/netboot/tasks/{id}", h.InstallTask.Update)
		r.Delete("/netboot/tasks/{id}", h.InstallTask.Delete)

		// Access control
		r.Get("/access/blacklist", h.Access.ListBlacklist)
		r.Post("/access/blacklist", h.Access.CreateBlacklist)
		r.Delete("/access/blacklist/{id}", h.Access.DeleteBlacklist)
		r.Get("/access/whitelist", h.Access.ListWhitelist)
		r.Post("/access/whitelist", h.Access.CreateWhitelist)
		r.Delete("/access/whitelist/{id}", h.Access.DeleteWhitelist)
		r.Get("/access/unauthorized", h.Access.ListUnauthorizedDevices)
		r.Post("/access/unauthorized/add-to-whitelist", h.Access.AddToWhitelistFromUnauthorized)
		r.Post("/access/unauthorized/add-to-blacklist", h.Access.AddToBlacklistFromUnauthorized)
		r.Delete("/access/unauthorized/{id}", h.Access.DeleteUnauthorizedDevice)

		// DNS records
		r.Get("/dns/records", h.DNSRecord.List)
		r.Post("/dns/records", h.DNSRecord.Create)
		r.Get("/dns/records/{id}", h.DNSRecord.Get)
		r.Put("/dns/records/{id}", h.DNSRecord.Update)
		r.Delete("/dns/records/{id}", h.DNSRecord.Delete)

		// BMC configs — import must be registered before {id}
		r.Get("/bmc/configs", h.BMC.List)
		r.Post("/bmc/configs", h.BMC.Create)
		r.Post("/bmc/configs/import", h.BMC.ImportCSV)
		r.Get("/bmc/configs/{id}", h.BMC.Get)
		r.Put("/bmc/configs/{id}", h.BMC.Update)
		r.Delete("/bmc/configs/{id}", h.BMC.Delete)

		// BMC batch operations (before {id}-based routes)
		r.Post("/bmc/batch/power-on", h.BMC.BatchPowerOn)
		r.Post("/bmc/batch/power-off", h.BMC.BatchPowerOff)
		r.Post("/bmc/batch/restart", h.BMC.BatchRestart)
		r.Post("/bmc/batch/status", h.BMC.BatchStatus)

		// BMC probe & power actions
		r.Post("/bmc/probe", h.BMC.Probe)
		r.Post("/bmc/{id}/refresh", h.BMC.Refresh)
		r.Post("/bmc/{id}/power-on", h.BMC.PowerOn)
		r.Post("/bmc/{id}/power-off", h.BMC.PowerOff)
		r.Post("/bmc/{id}/restart", h.BMC.PowerRestart)
		r.Get("/bmc/{id}/status", h.BMC.PowerStatus)
		r.Post("/bmc/{id}/boot-device", h.BMC.SetBootDevice)

		// DHCP reservations
		r.Get("/dhcp/reservations", h.DHCPReservation.List)
		r.Post("/dhcp/reservations", h.DHCPReservation.Create)
		r.Get("/dhcp/reservations/{id}", h.DHCPReservation.Get)
		r.Put("/dhcp/reservations/{id}", h.DHCPReservation.Update)
		r.Delete("/dhcp/reservations/{id}", h.DHCPReservation.Delete)

		// Network diagnostics
		r.Post("/network/ping", h.Network.Ping)
		r.Post("/network/ping/stream", h.Network.PingStream)
		r.Post("/network/traceroute", h.Network.Traceroute)
		r.Get("/network/interfaces", h.Network.ListInterfaces)

		// WOL
		r.Post("/hosts/batch/wake", h.WOL.BatchWake)
		r.Get("/wol/history", h.WOL.ListHistory)
		r.Get("/wol/history/{mac}", h.WOL.ListHistoryByMAC)
		r.Delete("/wol/history/{id}", h.WOL.DeleteHistory)
		r.Delete("/wol/history", h.WOL.DeleteAllHistory)
		r.Post("/wol/schedule", h.WOL.CreateSchedule)
		r.Get("/wol/schedules", h.WOL.ListSchedules)
		r.Delete("/wol/schedule/{id}", h.WOL.DeleteSchedule)
		r.Get("/wol/interfaces", h.WOL.ListInterfaces)

		// OS images
		r.Get("/os-images", h.OSImage.List)
		r.Post("/os-images/upload", h.OSImage.Upload)
		r.Get("/os-images/{id}", h.OSImage.Get)
		r.Delete("/os-images/{id}", h.OSImage.Delete)
		r.Post("/os-images/{id}/extract", h.OSImage.Extract)
		r.Post("/os-images/{id}/mount", h.OSImage.Mount)
		r.Post("/os-images/{id}/unmount", h.OSImage.Unmount)

		// Bootloader management
		r.Get("/bootloader/check", h.Bootloader.Check)
		r.Get("/bootloader/files", h.Bootloader.List)
		r.Post("/bootloader/check-file", h.Bootloader.CheckFile)


		// PXE runtime endpoints (no auth, registered in isPublicPath)
		r.Get("/netboot/task/by-mac/{mac}", h.InstallTask.GetTaskByMAC)
		r.Get("/netboot/answer/{task_id}", h.InstallTask.GetAnswerFile)
	})
}

func logDir(cfg *config.Config) string {
	dd := cfg.Global.DataDir
	if dd == "" {
		dd = ".pxelab"
	}
	if cfg.Log.File != "" {
		return filepath.Dir(cfg.Log.File)
	}
	return filepath.Join(dd, "logs")
}
