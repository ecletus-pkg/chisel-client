// +build !assetfs_bindata

package tunnel

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/moisespsena-go/env-utils"

	"github.com/moisespsena-go/error-wrap"

	"github.com/moisespsena-go/path-helpers"
)

func (p *Plugin) registerPrivatePaths() {
	var (
		goarch = envutils.Get("GO_DEST_ARCH", runtime.GOARCH)
		goos   = envutils.Get("GO_DEST_OS", runtime.GOOS)
	)

	pth := filepath.Join(path_helpers.GetCalledDir(true), "_private", "bin", goos, goarch)

	if goarch == "arm" {
		if armv := envutils.FistEnv("GO_DEST_ARM", "GOARM"); armv == "" {
			panic("Env GO_DEST_ARM and GOARM is blank")
		} else {
			pth = filepath.Join(pth, armv)
		}
	}

	if _, err := os.Stat(pth); os.IsNotExist(err) {
		panic(errwrap.Wrap(err, pth))
	}
	
	p.privateFS.RegisterPath(pth)
}
