package cofaas

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	//gen "github.com/truls/cofaas-go/protogen"
	cp "github.com/nmrshll/go-cp"
)

var requireUnimplemented *bool

// go generate go build -o cofaasgen -C protogen
func GenProtoCode(file string) (string, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "cofass-protogen")
	if err != nil {
		return "", err
	}
	fmt.Println(dir)
	//defer os.RemoveAll(dir)

	_, fname := filepath.Split(file)
	if !strings.HasSuffix(fname, ".proto") {
		return "", errors.New("Proto file %a must have suffix .proto")
	}
	pkg_name := strings.TrimSuffix(fname, ".proto")

	cp.CopyFile(file, filepath.Join(dir, fname))

	protoc_path, err := exec.LookPath("protoc")
	if err != nil {
		return "", err
	}

	protoc := exec.Command(protoc_path,
		"--plugin=protoc-gen-cofaas=protogen/cofaasgen",
		"-I"+dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+dir,
		fname)
	_, err = protoc.Output()
	if err, ok := err.(*exec.ExitError); ok {
		return "", errors.New(fmt.Sprintf("Failed with code %d\n%s",
			err.ProcessState.ExitCode(),
			err.Stderr))
	} else if err != nil {
		return "", err
	}

	res, err := os.ReadFile(filepath.Join(dir, pkg_name+"_grpc.pb.go"))
	if err != nil {
		return "", err
	}

	return string(res), nil

}
