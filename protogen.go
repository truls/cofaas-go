package cofaas

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/go-errors/errors"
	opt "github.com/moznion/go-optional"
	cp "github.com/otiai10/copy"
)

var requireUnimplemented *bool

type generator struct {
	dir      string
	pkg_name string
	fname    string
}

type wrapperScript string

func newWrapperScript(contents string) (wrapperScript, error) {
	f, err := os.CreateTemp(os.TempDir(), "*")
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer f.Close()
	fname := f.Name()
	if _, err := fmt.Fprintf(f, `#!/bin/bash
exec %s
`, contents); err != nil {
		os.Remove(fname)
		return "", errors.Wrap(err, 0)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		os.Remove(fname)
		return "", errors.Wrap(err, 0)
	}
	return *(*wrapperScript)(unsafe.Pointer(&fname)), nil
}

func (s wrapperScript) string() string {
	return string(s)
}

func (s wrapperScript) cleanup() {
	os.Remove(string(s))
}

func newGenerator(file string, otherFile opt.Option[string]) (*generator, error) {
	gen := generator{}

	dir, err := os.MkdirTemp("", "cofass-protogen")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	gen.dir = dir

	_, fname := filepath.Split(file)
	gen.fname = fname
	if !strings.HasSuffix(gen.fname, ".proto") {
		return nil, errors.Wrap("proto file %a must have suffix .proto", 0)
	}
	gen.pkg_name = strings.TrimSuffix(gen.fname, ".proto")

	if err := cp.Copy(file, filepath.Join(gen.dir, gen.fname)); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if otherFile.IsSome() {
		f := otherFile.Unwrap()
		if err := cp.Copy(f, filepath.Join(gen.dir, filepath.Base(f))); err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	return &gen, err
}

func (*generator) runProtoc(args ...string) error {
	protoc_path, err := exec.LookPath("protoc")
	if err != nil {
		return errors.Wrap(err, 0)
	}

	protoc := exec.Command(protoc_path, args...)
	output, err := protoc.CombinedOutput()
	if err_assert, ok := err.(*exec.ExitError); ok {
		return errors.Errorf("running command %s Failed with code %d\n%s",
			protoc.String(),
			err_assert.ExitCode(),
			output)
	} else if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func (g *generator) getOutputFile(suffix string) string {
	return g.pkg_name + suffix
}

func (g *generator) readOutput(fileName string) (string, error) {
	res, err := os.ReadFile(filepath.Join(g.dir, fileName))
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return string(res), nil
}

func (g *generator) cleanup() error {
	return os.RemoveAll(g.dir)
}

func GenGrpcCode(file string) (string, error) {
	g, err := newGenerator(file, nil)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer g.cleanup()
	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/grpc")
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer ws.cleanup()

	if err := g.runProtoc("--plugin=protoc-gen-cofaas="+ws.string(),
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname); err != nil {
		return "", errors.Wrap(err, 0)
	}

	return g.readOutput(g.getOutputFile("_grpc.pb.go"))
}

func GenProtoCode(file string) (string, error) {
	g, err := newGenerator(file, nil)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer g.cleanup()
	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/types")
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer ws.cleanup()

	if err := g.runProtoc("--plugin=protoc-gen-cofaas="+ws.string(),
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname); err != nil {
		return "", errors.Wrap(err, 0)
	}

	return g.readOutput(g.getOutputFile(".pb.go"))
}

func GenComponentCode(exportFile string, importFile opt.Option[string]) (string, error) {
	g, err := newGenerator(exportFile, importFile)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer g.cleanup()

	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/component")
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer ws.cleanup()
	fileArgs := []string{
		"--plugin=protoc-gen-cofaas=" + ws.string(),
		"-I" + g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out=" + g.dir,
		g.fname}

	if importFile.IsSome() {
		fileArgs = append(fileArgs, path.Base(importFile.Unwrap()))
	}

	if err := g.runProtoc(fileArgs...); err != nil {
		return "", errors.Wrap(err, 0)
	}

	return g.readOutput("component.go")
}
