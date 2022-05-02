package sls

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/rsb/failure"
)

const (
	GoBinaryName   = "go"
	GoBuildCmdName = "build"
	DefaultLDFlags = `-ldflags="-s -w -X main.Version=$(git rev-parse HEAD)"`
)

type CompileResult struct {
	BuildDir   string
	BinaryName string
	BinaryPath string
	CodeDir    string
}

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

// NewGoBuildCmd is designed to compile golang source code used for AWS Lambdas,
// for us these are features in our microservices.
// 	buildDir   - directory the binary will be compiled to
//  binaryName - is the name of the binary, this is used in the -o flag
//  targetDir  - is the directory which contains the source code to build
func NewGoBuildCmd(buildDir, binaryName, targetDir string) (*exec.Cmd, error) {
	if targetDir == "" {
		return nil, failure.System("[targetDir] is empty")
	}

	if _, err := os.Stat(targetDir); err != nil {
		return nil, failure.ToSystem(err, "os.Stat failed, targetDir does not exist or is not readable")
	}

	if _, err := os.Stat(buildDir); err != nil {
		return nil, failure.ToSystem(err, "os.Stat failed, buildDir does not exist or is not readable")
	}

	if binaryName == "" {
		return nil, failure.System("binaryName is empty")
	}

	goExec, err := exec.LookPath(GoBinaryName)
	if err != nil {
		return nil, failure.ToSystem(err, "exec.LookPath failed")
	}

	outputPath := fmt.Sprintf("%s/%s", buildDir, binaryName)

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
