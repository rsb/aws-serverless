package sls

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rsb/failure"
)

const (
	DefaultOutputName = "bootstrap"
	GoBuildCmdName    = "build"
	GoBinaryName      = "go"
	DefaultLDFlags    = `-ldflags="-s -w -X main.Version=$(git rev-parse HEAD)"`
)

func Zip(zf, binary string) error {
	zipfile, err := os.Create(zf)
	if err != nil {
		return failure.ToSystem(err, "os.Create failed")
	}
	defer func() {
		cerr := zipfile.Close()
		if cerr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[defer] Failed to close zip file: %v\n", cerr)
		}
	}()

	zw := zip.NewWriter(zipfile)
	defer func() {
		zerr := zw.Close()
		if zerr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[defer] Failed to close zip writer file: %v\n", zerr)
		}
	}()

	data, err := ioutil.ReadFile(binary)
	if err != nil {
		return failure.Wrap(err, "ioutil.ReadFile failed for (%s)", binary)
	}

	header := &zip.FileHeader{
		CreatorVersion: 3 << 8,     // indicated Unix
		ExternalAttrs:  0777 << 16, // -rwxrwxrwx file permissions
		Name:           "bootstrap",
		Method:         zip.Deflate,
	}
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return failure.ToSystem(err, "zw.CreateHeader failed")
	}

	if _, err := writer.Write(data); err != nil {
		return failure.ToSystem(err, "writer.Write failed")
	}

	return nil
}

func NewGoBuildCmd(outputDir, outputName, targetDir string) (*exec.Cmd, error) {
	if targetDir == "" {
		return nil, errors.New("[b.RootDir] is empty")
	}

	if _, err := os.Stat(targetDir); err != nil {
		return nil, errors.Wrapf(err, "os.Stat failed, b.TargetDir does not exist or is not readable")
	}

	if _, err := os.Stat(outputDir); err != nil {
		return nil, errors.Wrapf(err, "os.Stat failed, b.OutputDir does not exist or is not readable")
	}

	if outputName == "" {
		return nil, errors.New("outputName is empty")
	}

	goExec, err := exec.LookPath(GoBinaryName)
	if err != nil {
		return nil, errors.Wrap(err, "LookupGoBinary failed")
	}

	outputPath := filepath.Join(outputDir, outputName)

	cmd := exec.Cmd{
		Env:  append(os.Environ(), "GOOS=linux"),
		Dir:  targetDir,
		Path: goExec,
		Args: []string{
			goExec,
			GoBuildCmdName,
			DefaultLDFlags,
			"-o",
			outputPath,
			".",
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	return &cmd, nil
}
