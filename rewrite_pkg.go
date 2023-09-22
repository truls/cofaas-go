package cofaas

import (
	"fmt"
	"os"
	"path"
	"strings"

	//"github.com/gookit/goutil/dump"
	"github.com/go-errors/errors"
	"github.com/gookit/goutil/dump"
	cp "github.com/otiai10/copy"

	"github.com/truls/cofaas-go/metadata"
	"golang.org/x/mod/modfile"
	pkg "golang.org/x/tools/go/packages"
)

const (
	implPkgPath = "cofaas/application/impl"
	metadataFile = "cofaas_metadata.yaml"
)

type PkgRewriter struct {
	Metadata *metadata.Metadata
	pkg      *pkg.Package
	ModDir   string
}

// NewPkgRewriter copies the package found in modPath to baseDir,
// renames the package according to the cofaas module hierarchy and
// finally loads and parses the package. If this is successful, a
// PkgRewriter object is returned or otherwise an error
func NewPackageRewriter(modPath string, baseDir string) (*PkgRewriter, error) {
	implPath := path.Join(baseDir, "impl")

	if err := cp.Copy(modPath, implPath); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	p := PkgRewriter{ModDir: implPath}
	if err := p.renameModule(); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if err := p.loadPackage(); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if err := p.loadMetadata(modPath); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return &p, nil
}

// Modifies go.mod to rename package to cofaas/aplication/impl
func (p *PkgRewriter) renameModule() error {
	return p.transformMod(
		func(f *modfile.File) error {
			f.AddModuleStmt(implPkgPath)
			return nil
		})
}

func (p *PkgRewriter) transformMod(tf func(*modfile.File) error) error {
	modPath := path.Join(p.ModDir, "go.mod")
	c, err := os.ReadFile(modPath)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	f, err := modfile.Parse(p.ModDir, c, nil)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	if err := tf(f); err != nil {
		return errors.Wrap(err, 0)
	}
	newMod, err := f.Format()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	fmt.Printf("%s", string(newMod))
	if err := os.WriteFile(modPath, newMod, 0644); err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func (pr *PkgRewriter) loadMetadata(implPath string) error {
	m, err := metadata.Parse(path.Join(implPath, metadataFile), true)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	dump.Print(m)
	pr.Metadata = m
	return nil
}

func (pr *PkgRewriter) loadPackage() error {
	cfg := pkg.Config{
		Mode: pkg.NeedName | pkg.NeedFiles,
		Dir: pr.ModDir}

	pkgs, err := pkg.Load(&cfg, implPkgPath)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if len(pkgs) != 1 {
		return fmt.Errorf("input module must contain only a single package")
	}

	p := *pkgs[0]

	if len(p.Errors) > 0 {
		errs := strings.Builder{}
		errs.WriteString("Loading package failed\n")
		for _, e := range p.Errors {
			errs.WriteString(e.Error())
			errs.WriteString("\n")
		}
		return fmt.Errorf("%s", errs.String())
	}

	if p.Name != "main" {
		return fmt.Errorf("package must be named main not %s", p.Name)
	}
	pr.pkg = &p

	return nil
}

func (r *PkgRewriter) Rewrite(protoReplaements map[string]string) error {
	for _, n := range r.pkg.GoFiles {
		rewritten, err := NewSrcRewriter(protoReplaements).Rewrite(n)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if err := rewritten.Write(n); err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return nil
}