package conf

import (
	"github.com/spf13/pflag"

	"arhat.dev/aranya/pkg/constant"
)

type VirtualnodeNodeStorageConfig struct {
	Enabled                bool   `json:"enabled" yaml:"enabled"`
	RootDir                string `json:"rootDir" yaml:"rootDir"`
	KubeletPluginsDir      string `json:"kubeletPluginsDir" yaml:"kubeletPluginsDir"`
	KubeletRegistrationDir string `json:"kubeletRegistrationDir" yaml:"kubeletRegistrationDir"`
	SFTP                   struct {
		Enabled     bool   `json:"enabled" yaml:"enabled"`
		HostKeyFile string `json:"hostKey" yaml:"hostKey"`
	} `json:"sftp" yaml:"sftp"`
}

func FlagsForVirtualnodeNodeStorageConfig(prefix string, config *VirtualnodeNodeStorageConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode.node.storage", pflag.ExitOnError)

	flags.BoolVar(&config.Enabled, prefix+"enabled", false,
		"enable storage support in edge device by default")
	flags.StringVar(&config.RootDir, prefix+"rootDir",
		constant.DefaultAranyaStorageDir, "set dir to host pods volume mount")
	flags.StringVar(&config.KubeletPluginsDir, prefix+"kubeletPluginsDir",
		constant.DefaultKubeletPluginsDir, "set kubelet plugins dir")
	flags.StringVar(&config.KubeletRegistrationDir, prefix+"kubeletRegDir",
		constant.DefaultKubeletRegDir, "set kubelet plugin registration dir")

	// virtualnode.node.storage.sftp
	flags.BoolVar(&config.SFTP.Enabled, prefix+"sftp", true, "enable sftp server")
	flags.StringVar(&config.SFTP.HostKeyFile, prefix+"sftp.hostKeyFile",
		constant.DefaultSFTPHostKeyFile, "path to sftp host private key")

	return flags
}
