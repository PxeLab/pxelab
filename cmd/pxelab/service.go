package main

import (
	"os"

	"github.com/spf13/cobra"
)

// serviceName is the OS service name.
const serviceName = "PxeLab"

// serviceDisplayName is the human-readable service name.
const serviceDisplayName = "PxeLab PXE Server"

// serviceDesc is the service description.
const serviceDesc = "All-in-one PXE server (DHCP/TFTP/DNS/HTTP)"

// binPath returns the current executable path.
func binPath() string {
	p, err := os.Executable()
	if err != nil {
		return "PxeLab"
	}
	return p
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "管理系统服务",
	Long: `安装、移除、启动、停止 PxeLab 系统服务。

Windows: 使用 Windows Service Control Manager
Linux:   使用 systemd
macOS:   使用 launchd`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "安装系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgFile, _ := rootCmd.Flags().GetString("config")
		return serviceInstall(cfgFile)
	},
}

var serviceRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "移除系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serviceRemove()
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serviceStart()
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serviceStop()
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceRemoveCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	rootCmd.AddCommand(serviceCmd)
}
