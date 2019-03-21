# tunnel
https://github.com/mmatczuk/go-http-tunnel plugin for Aghape Framework

## Usage

Create subdirectory `tunnel` on your project config dir.
 
```bash
$ cd $PROJECT_DIR
$ cd config
$ mkdir tunnel
$ cd tunnel
```

Create the client TLS certificate:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -sha256 -keyout client.key -out client.crt
``` 

### Configuration

Create the tunnel configuration file `tunnel.yaml`. See [Configuration Details](https://github.com/mmatczuk/go-http-tunnel/blob/master/README.md#configuration).

If `Router.ServerAddr` is not zero and, `tunnel.Tunnels["main"]` is nil 
or tunnel `LocalAddr` is zero, the 
`LocalAddr` has be set to `"localhost:" + Router.ServerAddr.Ports()` and, if `Protocol` is empty set to `"tcp"`.  

Create the main configuration file `main.yaml`.

```yaml
auto_start: true
``` 

Options:

* `auto_start` bool: set to `true` for start tunnel client on apllication server start.
* `log_level`  int: set the log level. Value betwem 0 to 3.
* `log_stdout` string: stdout log file path. Use `"null"` for discard. Default is STDOUT.
* `log_stderr` string: stderr log file path. Use `"null"` for discard. Default is STDERR.
* `log_combined` bool: set to `true` for combine stderr to stdout.

### Register Plugin

```go
import "github.com/aghape-pkg/tunnel"

// ...
plugins = plugins.append(plugins, &tunnel.Plugin{ConfigDirKey: CONFIG_DIR, RouterKey: ROUTER})
// ...
```
