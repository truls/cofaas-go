package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	opt "github.com/moznion/go-optional"
	cp "github.com/otiai10/copy"
	c "github.com/truls/cofaas-go"
	"github.com/truls/cofaas-go/metadata"
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

var (
	pkgVersion = opt.Some("v0.0.0-20230922142509-34101b6cc96a")
	pkgReplacements = map[string]*c.PkgSpec{
		"google.golang.org/grpc": {
			Name:    "github.com/truls/cofaas-go/stubs/grpc",
			Version: pkgVersion,
			SubPkg: false},
		"google.golang.org/grpc/reflection": {
			Name:    "github.com/truls/cofaas-go/stubs/grpc/reflection",
			Version: pkgVersion,
			SubPkg: true},
		"google.golang.org/grpc/credentials/insecure": {
			Name:    "github.com/truls/cofaas-go/stubs/grpc/credentials/insecure",
			Version: pkgVersion,
			SubPkg: true},
		"net": {
			Name:    "github.com/truls/cofaas-go/stubs/net",
			Version: pkgVersion,
			SubPkg: false},
	}
)

type goDep struct {
	// Import path of the dependency
	// "github.com/truls/cofaas-go/stubs/grpc",
	// "github.com/truls/cofaas-go/stubs/net",
	importPath string
	// Version of the dependency
	version opt.Option[string]
}

type goModule struct {
	replacements map[string]string
	transformer  *transformer
	targetDir    string
	name         c.CofaasName
	goExec       string
	dependency   []goDep
}

type transformer struct {
	deferredGoCommands []*exec.Cmd
}

type implPacakge struct {
	mod                  *goModule
	meta                 *metadata.Metadata
	rwr                  *c.PkgRewriter
	protoPkgReplacements c.PkgReplacement
}

func newTransfoermer() *transformer {
	return &transformer{deferredGoCommands: []*exec.Cmd{}}
}

func (t *transformer) finalize() error {
	for _, cmd := range t.deferredGoCommands {
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to run command %s: %v, with output \n\n%s", cmd.String(), err, output)
		}
	}
	return nil
}

func newGoModule(moduleName c.CofaasName, targetDir string, t *transformer) (*goModule, error) {
	go_exec, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("could not find go executable: %v", err)
	}

	return &goModule{
		name:         moduleName,
		dependency:   []goDep{},
		replacements: make(map[string]string),
		targetDir:    targetDir,
		goExec:       go_exec,
		transformer:  t,
	}, nil
}

func (m *goModule) writeFile(name string, contents string) error {
	return os.WriteFile(path.Join(m.targetDir, name), []byte(contents), 0644)
}

func (m *goModule) deferGoCommand(args ...string) {
	gocmd := exec.Command(m.goExec, args...)
	gocmd.Dir = m.targetDir
	t := m.transformer
	t.deferredGoCommands = append(t.deferredGoCommands, gocmd)
}

// Runs go with specified arguments in the working directory of the module
func (m *goModule) runGoCommand(args ...string) error {
	gocmd := exec.Command(m.goExec, args...)
	gocmd.Dir = m.targetDir
	//fmt.Println("Running command in " + m.targetDir)
	if output, err := gocmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run command %s: %v, with output \n\n%s", gocmd.String(), err, output)
	}

	return nil
}

func (m *goModule) addReplacement(from c.CofaasName, to string) {
	m.replacements[from.String()] = to
}

func (m *goModule) create() error {
	if err := m.runGoCommand("mod", "init", m.name.String()); err != nil {
		return errors.Wrap(err, 0)
	}

	return m.tidy()
}

func (m *goModule) tidy() error {
	for k, v := range m.replacements {
		if err := m.runGoCommand("mod", "edit",
			fmt.Sprintf("-replace=%s=%s", k, v)); err != nil {
			return errors.Wrap(err, 0)
		}
	}

	for _, d := range m.dependency {
		version := d.version.TakeOr("v0.0.0")
		if err := m.runGoCommand("mod", "edit", "-require", fmt.Sprintf("%s@%s", d.importPath, version)); err != nil {
			return errors.Wrap(err, 0)
		}
	}

	// Tidy commands must be run after all modules have been generated
	m.deferGoCommand("mod", "tidy")

	return nil
}

func getProtoBaseName(protoPath string) (string, error) {
	protoBaseName := strings.Split(path.Base(protoPath), ".")[0]
	if protoBaseName == "" {
		return "", fmt.Errorf("unable to extract proto name from path %s", protoPath)
	}
	return protoBaseName, nil
}

func (t *transformer) genProtoModule(moduleBase string, protoFile string) (c.CofaasName, error) {
	moduleBase = path.Join(moduleBase, "protos")
	stat, err := os.Stat(moduleBase)
	if err == nil && !stat.IsDir() {
		return "", fmt.Errorf("path %s exists but is not a directory", moduleBase)
	} else if os.IsNotExist(err) {
		if err := os.Mkdir(moduleBase, 0755); err != nil {
			return "", errors.Wrap(err, 0)
		}
	}

	protoBaseName, err := getProtoBaseName(protoFile)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	modulePath := path.Join(moduleBase, protoBaseName)
	if err := os.Mkdir(modulePath, 0755); err != nil {
		return "", errors.Errorf("unable to create directory %s: %v", modulePath, err)
	}

	modName := c.ProtoNameBase.Ident(protoBaseName)

	m, err := newGoModule(modName, modulePath, t)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	res, err := c.GenGrpcCode(protoFile)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	m.writeFile("grpc.go", res)

	res, err = c.GenProtoCode(protoFile)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	m.writeFile("proto.go", res)

	return modName, m.create()
}

func (t *transformer) genProtoComponent(
	moduleBase string,
	meta *metadata.Metadata,
	witPath string,
	witWorld string) error {

	moduleBase = path.Join(moduleBase, "component")
	if err := os.Mkdir(moduleBase, 0755); err != nil {
		return errors.Wrap(err, 0)
	}

	m, err := newGoModule(c.ComponentName, moduleBase, t)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	res, err := c.GenComponentCode(
		meta.ExportProto.Path,
		opt.Map(meta.ImportProto,
			func(x *metadata.ProtoSpec) string { return x.Path }))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	m.writeFile("component.go", res)

	witPathAbs, err := filepath.Abs(witPath)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	// Run wit-bindgen
	witBindgen := exec.Command("wit-bindgen", "tiny-go", witPathAbs, "--world", witWorld, "--out-dir=gen")
	witBindgen.Dir = moduleBase
	if res, err := witBindgen.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run wit-bindgen: %v\n\n%s", err, res)
	}

	m.addProtoReplacements(meta)
	m.addReplacement(c.ImplName, "../impl")

	return m.create()
}

// addProtoReplacements configures grpc replacement path based on
// metadata derived from the module to be transformed
func (m *goModule) addProtoReplacements(meta *metadata.Metadata) error {
	ar := func(s *metadata.ProtoSpec) {
		m.addReplacement(c.ProtoNameBase.Ident(s.Name), "../protos/"+s.Name)
	}
	ar(meta.ExportProto)
	if meta.ImportProto.IsSome() {
		ar(meta.ImportProto.Unwrap())
	}
	return nil
}

func (t *transformer) newImpl(dir string, pkgDir string, exportProt string, importProto opt.Option[string]) (*implPacakge, error) {
	rwr, err := c.NewPackageRewriter(pkgDir, dir)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	m, err := newGoModule(c.ImplName, rwr.ModDir, t)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	m.addProtoReplacements(rwr.Metadata)


	return &implPacakge{
		mod:                  m,
		meta:                 rwr.Metadata,
		rwr:                  rwr,
		protoPkgReplacements: pkgReplacements,
	}, nil
}

func (i *implPacakge) addImportReplacement(im string, replacement string, version opt.Option[string]) {
	i.protoPkgReplacements[im] = &c.PkgSpec{
		Name:    replacement,
		Version: version}
}

func (i *implPacakge) finalize() error {
	for _, v := range i.protoPkgReplacements {
		if ! v.SubPkg {
			i.mod.runGoCommand("mod", "edit", "-require="+v.Format())
		}
	}

	if err := i.mod.tidy(); err != nil {
		return errors.Wrap(err, 0)
	}

	if err := i.rwr.Rewrite(i.protoPkgReplacements); err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func doTransform(exportProto string, importProto opt.Option[string], outputDir string, witPath string, witWorld string, implPath string) error {
	dir, err := os.MkdirTemp(os.TempDir(), "cofaas-transform")
	fmt.Println(dir)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	// Remove temporary directory in case of failure
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	t := newTransfoermer()

	implPkg, err := t.newImpl(dir, implPath, exportProto, importProto)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if n, err := t.genProtoModule(dir, exportProto); err != nil {
		return errors.Wrap(err, 0)
	} else {
		implPkg.addImportReplacement(implPkg.meta.ExportProto.Import, n.String(), nil)
	}

	if importProto.IsSome() {
		if n, err := t.genProtoModule(dir, importProto.Unwrap()); err != nil {
			return errors.Wrap(err, 0)
		} else {
			implPkg.addImportReplacement(implPkg.meta.ImportProto.Unwrap().Import, n.String(), nil)
		}
	}

	if err := t.genProtoComponent(dir, implPkg.meta, witPath, witWorld); err != nil {
		return errors.Wrap(err, 0)
	}

	if err := implPkg.finalize(); err != nil {
		return errors.Wrap(err, 0)
	}

	// Run deferred go mod tidy commands
	if err := t.finalize(); err != nil {
		return errors.Wrap(err, 0)
	}

	// Finally move temporary directory to destination
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return cp.Copy(dir, absDir)
}

func main() {
	exportProto := flag.String("exportProto", "", "The export protocol file name")
	importProto := flag.String("importProto", "", "The import protocol file name")
	outputDir := flag.String("outputDir", "", "The output directory")
	// compileComponent := flag.Bool("compileComponent", true, "Compile the wasm components")
	// keepCode := flag.Bool("keepCode", true, "Keep the transformed code")
	witPath := flag.String("witPath", "", "The directory containing wit files")
	witWorld := flag.String("witWorld", "", "The WIT world to generate a component for")
	implPath := flag.String("implPath", "", "Path to the implementation")
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

	if *implPath == "" {
		fmt.Println("Flag implPath must be set")
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

	ip := opt.Some(*importProto)
	if *importProto == "" {
		ip = opt.None[string]()
	}

	if err := doTransform(*exportProto, ip, *outputDir, *witPath, *witWorld, *implPath); err != nil {
		fmt.Printf("Generating go module failed %s\n", c.FormatError(err))
		os.Exit(1)
	}
}
