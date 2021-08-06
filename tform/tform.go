package tform

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-exec/tfinstall"
	"github.com/rsb/failure"
)

type Installer struct{}

func (c *Installer) Install(ctx context.Context, installDir, version string) (string, error) {
	if installDir == "" {
		return "", failure.Validation("installDir is empty")
	}

	installInfo, err := os.Stat(installDir)
	if os.IsNotExist(err) {
		return "", failure.ToNotFound(err, "install directory (%s)", installDir)
	}

	if !installInfo.IsDir() {
		return "", failure.System("install directory (%s) is not a directory", installDir)
	}

	var args []tfinstall.ExecPathFinder

	switch {
	case version == "" || version == "latest":
		finder := tfinstall.LatestVersion(installDir, false)
		finder.UserAgent = "tele-cli"
		args = append(args, finder)
	default:
		if strings.HasPrefix(version, "v") {
			version = version[1:]
		}
		finder := tfinstall.ExactVersion(version, installDir)
		finder.UserAgent = "tele-cli"
		args = append(args, finder)
	}

	path, err := tfinstall.Find(ctx, args...)
	if err != nil {
		return "", failure.ToSystem(err, "tfinstall.Find failed")
	}

	return path, nil
}

func (c *Installer) VerifyExistingInstall(ctx context.Context, path string) error {
	finder := tfinstall.ExactPath(path)

	if _, err := tfinstall.Find(ctx, finder); err != nil {
		return failure.ToSystem(err, "tfinstall.Find failed for (%s)", path)
	}

	return nil
}

func (c *Installer) EnsureInstallDir(dir string) (string, bool, error) {
	info, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return "", false, failure.ToSystem(err, "os.Stat failed for (%s)", dir)
	}

	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0755); err != nil {
			return "", false, failure.ToSystem(err, "os.Mkdir failed for (%s)", dir)
		}
		return dir, false, nil
	}

	if !info.IsDir() {
		return "", false, failure.System("install dir (%s) is not a directory", dir)

	}
	return dir, true, nil
}
