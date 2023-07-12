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

type generator struct {
	dir string
	pkg_name string
	fname string
}

func newGenerator(file string) (*generator, error) {
	gen := generator{}

	dir, err := ioutil.TempDir(os.TempDir(), "cofass-protogen")
	if err != nil {
		return nil, err
	}
	gen.dir = dir
	//fmt.Println(dir)

	_, fname := filepath.Split(file)
	gen.fname = fname
	if !strings.HasSuffix(gen.fname, ".proto") {
		return nil, errors.New("Proto file %a must have suffix .proto")
	}
	gen.pkg_name = strings.TrimSuffix(gen.fname, ".proto")

	err = cp.CopyFile(file, filepath.Join(gen.dir, gen.fname))
	return &gen, err

}

func (*generator) runProtoc(args ...string) error {
	protoc_path, err := exec.LookPath("protoc")
	if err != nil {
		return err
	}

	protoc := exec.Command(protoc_path, args...)
	fmt.Println(protoc.String())
	_, err = protoc.Output()
	if err, ok := err.(*exec.ExitError); ok {
		return errors.New(fmt.Sprintf("Failed with code %d\n%s",
			err.ProcessState.ExitCode(),
			err.Stderr))
	} else if err != nil {
		return err
	}
	return nil
}

func (g *generator) readOutput(suffix string) (string, error){
	res, err := os.ReadFile(filepath.Join(g.dir, g.pkg_name+suffix))
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func (g *generator) cleanup() error {
	return os.RemoveAll(g.dir)
}


//go:generate go build  -C protogen/grpc -o cofaasgen
func GenGrpcCode(file string) (string, error) {
	g, err := newGenerator(file)
	if err != nil {
		return "", err
	}
	defer g.cleanup()
	err = g.runProtoc("--plugin=protoc-gen-cofaas=protogen/grpc/cofaasgen",
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname)
	if err != nil {
		return "", err
	}


	res, err := g.readOutput("_grpc.pb.go")
	return res, err

}

//go:generate go build  -C protogen/types -o cofaasgen
func GenProtoCode (file string) (string, error) {
	g, err := newGenerator(file)
	if err != nil {
		return "", err
	}
	defer g.cleanup()
	err = g.runProtoc("--plugin=protoc-gen-cofaas=protogen/types/cofaasgen",
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname)
	if err != nil {
		return "", err
	}

	return g.readOutput(".pb.go")
}
