package tunnel

import (
	"github.com/mmatczuk/go-http-tunnel/cli/tunnel"
)

type MainConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ServerIndex int    `yaml:"server_index"`
	AutoStart   bool   `yaml:"auto_start"`
	LogLevel    int    `yaml:"log_level"`
	LogStdout   string `yaml:"log_stdout"`
	LogStderr   string `yaml:"log_stderr"`
	LogCombined bool   `yaml:"log_combined"`
}

type Config struct {
	Main   *MainConfig
	Tunnel *tunnel.ClientConfig
}
