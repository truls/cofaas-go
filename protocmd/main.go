package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	opt "github.com/moznion/go-optional"
	cp "github.com/otiai10/copy"
	c "github.com/truls/cofaas-go"
)

const cmdDescr = `Transforms a go module to a gofaas optimized module

For a go module in the directory a using proto b the following
hierarchy is generated

Original hierarchy
a
| a.go
| ...
| go.mod
| go.sum

Generated hierarchy
a
| proto -
|       | b.proto
|       | g_grpc.proto
|       | go.mod
|       | go.sum
| component -
|           | component.go
|           | gp.mpd
|           | go.sum
| impl -
|      | a.go
|      | ...
|      | go.mod
|      | go.sum`

type goModule struct {
	name         string
	dependency   []string
	replacements map[string]string
	targetDir    string

	goExec string
}

func newGoModule(moduleName string, targetDir string) (*goModule, error) {
	go_exec, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("Could not find go executable: %v", err)
	}

	return &goModule{
		name:         moduleName,
		dependency:   []string{},
		replacements: make(map[string]string),
		targetDir:    targetDir,
		goExec:       go_exec,
	}, nil
}

func (m *goModule) writeFile(name string, contents string) error {
	return os.WriteFile(path.Join(m.targetDir, name), []byte(contents), 0644)
}

// Runs go with specified arguments in the working directory of the module
func (m *goModule) runGoCommand(args ...string) error {
	gocmd := exec.Command(m.goExec, args...)
	gocmd.Dir = m.targetDir
	if output, err := gocmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to run command %s: %v, with output \n\n%s", gocmd.String(), err, output)
	}
	return nil
}

func (m *goModule) addReplacement(from string, to string) {
	m.replacements[from] = to
}

func (m *goModule) create() error {
	if err := m.runGoCommand("mod", "init", m.name); err != nil {
		return err
	}

	for k, v := range m.replacements {
		if err := m.runGoCommand("mod", "edit",
			fmt.Sprintf("-replace=%s=%s", k, v)); err != nil {
			return err
		}
	}

	if err := m.runGoCommand("mod", "tidy"); err != nil {
		return err
	}

	return nil
}

func genProtoModule(moduleBase string, protoFile string) error {
	moduleBase = path.Join(moduleBase, "proto")
	stat, err := os.Stat(moduleBase)
	if err == nil && !stat.IsDir() {
		return fmt.Errorf("Path %s exists but is not a directory", moduleBase)
	} else if os.IsNotExist(err) {
		if err := os.Mkdir(moduleBase, 0755); err != nil {
			return err
		}
	}

	protoBaseName := strings.Split(path.Base(protoFile), ".")[0]
	if protoBaseName == "" {
		return fmt.Errorf("Unable to extract proto name from path %s", protoFile)
	}

	modulePath := path.Join(moduleBase, protoBaseName)
	if err := os.Mkdir(modulePath, 0755); err != nil {
		return fmt.Errorf("Unable to create directory %s: %v", modulePath, err)
	}

	m, err := newGoModule("cofaas/proto/"+protoBaseName, modulePath)
	if err != nil {
		return err
	}

	res, err := c.GenGrpcCode(protoFile)
	if err != nil {
		return err
	}
	m.writeFile("grpc.go", res)

	res, err = c.GenProtoCode(protoFile)
	if err != nil {
		return err
	}
	m.writeFile("proto.go", res)

	return m.create()
}

func genProtoComponent(
	moduleBase string,
	exportProto string,
	importProto opt.Option[string],
	witPath string,
	witWorld string) error {

	moduleBase = path.Join(moduleBase, "component")
	if err := os.Mkdir(moduleBase, 0755); err != nil {
		return err
	}

	m, err := newGoModule("cofaas/application/component", moduleBase)
	if err != nil {
		return err
	}

	res, err := c.GenComponentCode(exportProto, importProto)
	if err != nil {
		return err
	}
	m.writeFile("component.go", res)

	witPathAbs, err := filepath.Abs(witPath)
	if err != nil {
		return err
	}
	// Run wit-bindgen
	witBindgen := exec.Command("wit-bindgen", "tiny-go", witPathAbs, "--world", witWorld, "--out-dir=gen")
	witBindgen.Dir = moduleBase
	if res, err := witBindgen.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to run wit-bindgen: %v\n\n%s", err, res)
	}

	return m.create()
}

func doTransform(exportProto string, importProto opt.Option[string], outputDir string, witPath string, witWorld string) error {
	dir, err := ioutil.TempDir(os.TempDir(), "cofaas-transform")
	if err != nil {
		return err
	}

	// Remove temporary directory in case of failure
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	if err := genProtoModule(dir, exportProto); err != nil {
		return err
	}

	if err := genProtoComponent(dir, exportProto, importProto, witPath, witWorld); err != nil {
		return err
	}

	// m := newGoModule()
	// m.setName("cofaas/application/component")
	// m.create()

	// m = newGoModule()
	// m.setName("cofaas/application/impl")

	// Finally move temporary directory to destination
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	return cp.Copy(dir, absDir)
}

func main() {
	exportProto := flag.String("exportProto", "", "The export protocol file name")
	importProto := flag.String("importProto", "", "The import protocol file name")
	outputDir := flag.String("outputDir", "", "The output directory")
	witPath := flag.String("witPath", "", "The directory containing wit files")
	witWorld := flag.String("witWorld", "", "The WIT world to generate a component for")
	help := flag.Bool("help", false, "Prints help")
	flag.Parse()

	if *help {
		fmt.Println(cmdDescr)
		os.Exit(0)
	}

	if *exportProto == "" {
		fmt.Println("Flag protoFile must be set")
		flag.Usage()
		os.Exit(1)
	}

	if *outputDir == "" {
		fmt.Println("Flag outputDir must be set")
		flag.Usage()
		os.Exit(1)
	}

	if *witPath == "" {
		fmt.Println("Flag witPath must be set")
		flag.Usage()
		os.Exit(1)
	}

	if *witWorld == "" {
		fmt.Println("Flag witWorld must be set")
		flag.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(*outputDir); err == nil {
		fmt.Printf("DIrectory %v already exists. Specify a non-existant directory\n", *outputDir)
		os.Exit(1)
	}

	if _, err := os.Stat(*exportProto); os.IsNotExist(err) {
		fmt.Printf("File %v does not exists.\n", *exportProto)
		os.Exit(1)
	}

	err := doTransform(*exportProto, opt.FromNillable(importProto), *outputDir, *witPath, *witWorld)

	if err != nil {
		fmt.Printf("Generating go module failed %v\n", err)
		os.Exit(1)
	}

}
