package cofaas

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unsafe"

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
	f, err := ioutil.TempFile(os.TempDir(), "*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	fname := f.Name()
	if _, err := f.Write([]byte(fmt.Sprintf(`#!/bin/bash
exec %s
`, contents))); err != nil {
		os.Remove(fname)
		return "", err
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		os.Remove(fname)
		return "", err
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

	if err := cp.Copy(file, filepath.Join(gen.dir, gen.fname)); err != nil {
		return nil, err
	}
	if otherFile.IsSome() {
		f := otherFile.Unwrap()
		if err := cp.Copy(f, filepath.Join(gen.dir, filepath.Base(f))); err != nil {
			return nil, err
		}
	}

	return &gen, err
}

func (*generator) runProtoc(args ...string) error {
	protoc_path, err := exec.LookPath("protoc")
	if err != nil {
		return err
	}

	protoc := exec.Command(protoc_path, args...)
	output, err := protoc.CombinedOutput()
	if err, ok := err.(*exec.ExitError); ok {
		return errors.New(fmt.Sprintf("Running command %s Failed with code %d\n%s",
			protoc.String(),
			err.ProcessState.ExitCode(),
			output))
	} else if err != nil {
		return err
	}
	return nil
}

func (g *generator) getOutputFile(suffix string) string {
	return g.pkg_name + suffix
}

func (g *generator) readOutput(fileName string) (string, error) {
	res, err := os.ReadFile(filepath.Join(g.dir, fileName))
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func (g *generator) cleanup() error {
	return os.RemoveAll(g.dir)
}

func GenGrpcCode(file string) (string, error) {
	g, err := newGenerator(file, nil)
	if err != nil {
		return "", err
	}
	defer g.cleanup()
	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/grpc")
	if err != nil {
		return "", err
	}
	defer ws.cleanup()
	err = g.runProtoc("--plugin=protoc-gen-cofaas="+ws.string(),
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname)
	if err != nil {
		return "", err
	}

	return g.readOutput(g.getOutputFile("_grpc.pb.go"))
}

func GenProtoCode(file string) (string, error) {
	g, err := newGenerator(file, nil)
	if err != nil {
		return "", err
	}
	defer g.cleanup()
	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/types")
	if err != nil {
		return "", err
	}
	defer ws.cleanup()
	err = g.runProtoc("--plugin=protoc-gen-cofaas="+ws.string(),
		"-I"+g.dir,
		"--cofaas_opt=paths=source_relative",
		"--cofaas_out="+g.dir,
		g.fname)
	if err != nil {
		return "", err
	}

	return g.readOutput(g.getOutputFile(".pb.go"))
}

func GenComponentCode(exportFile string, importFile opt.Option[string]) (string, error) {
	g, err := newGenerator(exportFile, importFile)
	if err != nil {
		return "", err
	}
	defer g.cleanup()
	ws, err := newWrapperScript("go run github.com/truls/cofaas-go/protogen/component")
	if err != nil {
		return "", err
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

	err = g.runProtoc(fileArgs...)
	if err != nil {
		return "", err
	}

	return g.readOutput("component.go")
}
