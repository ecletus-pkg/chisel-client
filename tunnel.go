package tunnel

import (
	"fmt"
	"github.com/moisespsena/go-assetfs/assetfsapi"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/moisespsena-go/task"

	"github.com/moisespsena/go-assetfs"
	"gopkg.in/yaml.v2"

	"github.com/moisespsena/go-pluggable"

	"github.com/moisespsena/go-assetfs/api"

	"github.com/ecletus/router"
	"github.com/mmatczuk/go-http-tunnel/cli/tunnel"
	"github.com/moisespsena-go/default-logger"
	"github.com/moisespsena-go/error-wrap"
	"github.com/moisespsena-go/path-helpers"

	"github.com/ecletus/ecletus"
	"github.com/ecletus/cli"
	"github.com/ecletus/plug"
	"github.com/spf13/cobra"
)

const (
	CONFIG_DIR         = "tunnel"
	TUNNEL_CONFIG_FILE = CONFIG_DIR + string(os.PathSeparator) + "tunnel.yaml"
	CONFIG_FILE        = CONFIG_DIR + string(os.PathSeparator) + "main.yaml"
)

var (
	PTH = path_helpers.GetCalledDir()
	log = defaultlogger.NewLogger(PTH)
)

func pathOf(cmd *cobra.Command) []string {
	var pth []string

	for cmd != nil {
		pth = append(pth, cmd.Name())
		cmd = cmd.Parent()
	}

	last := len(pth) - 1
	for i := 0; i < len(pth)/2; i++ {
		pth[i], pth[last-i] = pth[last-i], pth[i]
	}

	return pth
}

func MakeCmd(fs assetfsapi.Interface, cfg *Config, onDone func(...func()), args ...string) (cmd *exec.Cmd, err error) {
	if len(args) == 0 {
		args = append(args, "start-all")
	}

	if cfg.Main.LogLevel != 0 {
		var has bool
		for _, arg := range args {
			if arg == "-logLevel" {
				has = true
				break
			}
		}
		if !has {
			args = append(args, "-logLevel", fmt.Sprint(cfg.Main.LogLevel))
		}
	}

	args = append([]string{"-config", "-"}, args...)

	tunnelFile := "tunnel"

	var cwd, _ = os.Getwd()
	tunnelFile = cwd + string(os.PathSeparator) + tunnelFile
	assetfs.SaveExecutable("tunnel", fs, tunnelFile, false)

	cmd = exec.Command(tunnelFile, args...)

	if fname := cfg.Main.LogStdout; fname != "" {
		switch fname {
		case "-":
			cmd.Stdout = ioutil.Discard
		default:
			if stdout, err := os.Create(fname); err != nil {
				panic(errwrap.Wrap(err, "Open stdout %q", fname))
			} else {
				cmd.Stdout = stdout
				onDone(func() {
					stdout.Close()
				})
			}
		}
	} else {
		cmd.Stdout = os.Stdout
	}

	if cfg.Main.LogCombined {
		cmd.Stderr = cmd.Stdout
	} else if fname := cfg.Main.LogStderr; fname != "" {
		switch fname {
		case "-":
			cmd.Stdout = ioutil.Discard
		default:
			if stderr, err := os.Create(fname); err != nil {
				panic(errwrap.Wrap(err, "Open stderr %q", fname))
			} else {
				cmd.Stderr = stderr
				onDone(func() {
					stderr.Close()
				})
			}
		}
	} else {
		cmd.Stderr = os.Stderr
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var (
		inBytes []byte
		in      io.WriteCloser
	)

	if in, err = cmd.StdinPipe(); err != nil {
		return nil, errwrap.Wrap(err, "Stdin pipe")
	}

	if inBytes, err = yaml.Marshal(&cfg.Tunnel); err != nil {
		return nil, errwrap.Wrap(err, "Marshal tunnel config")
	}

	in.Write(inBytes)
	in.Close()
	return cmd, nil
}

func Run(cmd *exec.Cmd) (err error) {
	if err := cmd.Start(); err != nil {
		return errwrap.Wrap(err, "cmd.Start() failed")
	}
	return
}

func CreateCommand(fs assetfsapi.Interface, cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Initialize tunnel Client",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) == 0 {
				args = append(args, "start-all")
			}
			args = append([]string{strings.Join(pathOf(cmd), " ")}, args...)
			var c *exec.Cmd
			if c, err = MakeCmd(fs, cfg, func(f ...func()) {}, args...); err != nil {
				return err
			}
			if err := Run(c); err != nil {
				return err
			}
			return c.Wait()
		},
	}
	return cmd
}

func LoadConfig(configDir *ecletus.ConfigDir) (cfg *Config, err error) {
	cfg = &Config{Main: &MainConfig{}}
	if tunnelConfig, err := tunnel.LoadClientConfigFromFile(configDir.Path(TUNNEL_CONFIG_FILE)); err != nil {
		return nil, errwrap.Wrap(err, "Load config file %q", TUNNEL_CONFIG_FILE)
	} else {
		cfg.Tunnel = tunnelConfig
	}

	if err := configDir.Load(cfg.Main, CONFIG_FILE); err != nil {
		return nil, errwrap.Wrap(err, "Load config file %q", CONFIG_FILE)
	}
	return
}

type Plugin struct {
	plug.EventDispatcher
	ConfigDirKey string
	PreConfigKey string
	RouterKey    string
	running      bool
	cfg          *Config
	cmd          *cobra.Command
	privateFS    assetfsapi.Interface
}

func (p *Plugin) RequireOptions() []string {
	return []string{p.RouterKey}
}

func (p *Plugin) OnRegister(options *plug.Options) {
	plug.OnFS(p, func(e *pluggable.FSEvent) {
		p.privateFS = e.PrivateFS
		p.registerPrivatePaths()
	})
	cli.OnRegister(p, func(e *cli.RegisterEvent) {
		p.cli(e)
	})
}

func (p *Plugin) Init(options *plug.Options) {
	p.cfg = p.loadConfig(options)
	if p.cfg != nil && p.cfg.Main.AutoStart {
		r := options.GetInterface(p.RouterKey).(*router.Router)
		srv := r.Config.Servers[p.cfg.Main.ServerIndex]
		if srv.Addr != "" {
			r.PreServe(func(r *router.Router, ta task.Appender) {
				var (
					agp = options.GetInterface(ecletus.AGHAPE).(*ecletus.Ecletus)
				)

				if p.cmd != nil {
					if cmd, err := MakeCmd(p.privateFS, p.cfg, func(f ...func()) {
						agp.PostRun(f...)
					}); err != nil {
						log.Error(err)
					} else {
						if err = agp.AddTask(&task.CmdTask{cmd, log}); err != nil {
							panic(err)
						}
					}
				}
			})
		}
	}
}

func (p *Plugin) loadConfig(options *plug.Options) (cfg *Config) {
	configDir := options.GetInterface(p.ConfigDirKey).(*ecletus.ConfigDir)
	if ok, err := configDir.Exists(TUNNEL_CONFIG_FILE); err != nil {
		log.Error(errwrap.Wrap(err, "Stat of %q", configDir.Path(TUNNEL_CONFIG_FILE)))
		return nil
	} else if !ok {
		log.Warning("SKIP because config is not valid.")
		return
	}
	var err error
	if cfg, err = LoadConfig(configDir); err != nil {
		log.Error(errwrap.Wrap(err, "load config"))
		return nil
	}
	if cfg.Tunnel.TLSCrt == "" {
		cfg.Tunnel.TLSCrt = configDir.Path(CONFIG_DIR, "client.crt")
	}
	if cfg.Tunnel.TLSKey == "" {
		cfg.Tunnel.TLSKey = configDir.Path(CONFIG_DIR, "client.key")
	}
	p.prepare(cfg, options)
	return cfg
}

func (p *Plugin) prepare(cfg *Config, options *plug.Options) {
	if cfg.Tunnel.Tunnels == nil {
		cfg.Tunnel.Tunnels = map[string]*tunnel.Tunnel{}
	}

	if p.PreConfigKey != "" {
		pf := options.GetInterface(p.PreConfigKey).(func(cfg *Config))
		pf(cfg)
	}

	r := options.GetInterface(p.RouterKey).(*router.Router)

	if cfg.Main.Enabled {
		t := cfg.Tunnel.Tunnels["main"]
		if t == nil {
			t = &tunnel.Tunnel{}
			cfg.Tunnel.Tunnels["main"] = t
		}

		t.Protocol = "tcp"
		t.LocalAddr = string(r.Config.Servers[cfg.Main.ServerIndex].Addr)
	}
}

func (p *Plugin) cli(e *cli.RegisterEvent) {
	if p.cfg != nil {
		cmd := CreateCommand(p.privateFS, p.cfg)
		e.RootCmd.AddCommand(cmd)
		p.cmd = cmd
	}
}
