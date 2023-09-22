package cofaas

import (
	"bytes"
	"fmt"

	//"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"

	//"github.com/gookit/goutil/dump"
	"github.com/go-errors/errors"
	"github.com/gookit/goutil/dump"
	"golang.org/x/tools/go/ast/astutil"
)

// https://eli.thegreenplace.net/2021/rewriting-go-source-code-with-ast-tooling/

var extraPackages = []string{
	"github.com/truls/cofaas-go/stubs/grpc",
	"github.com/truls/cofaas-go/stubs/net",
}

type srcRewriter struct {
	Rewriter
	protoImportReplacements map[string]string
}

type srcRewritten struct {
	Rewritten
	fset     *token.FileSet
	ast_file *ast.File
}

func NewSrcRewriter(protoImportReplacements map[string]string) Rewriter {
	return &srcRewriter{
		protoImportReplacements: protoImportReplacements,
	}
}

func (r *srcRewriter) applyFunction(c *astutil.Cursor) bool {
	n := c.Node()

	switch x := n.(type) {
	case *ast.FuncDecl:
		id := x.Name
		// Export main function
		if id.Name == "main" {
			x.Name.Name = "Main"
			c.Replace(x)
		}
	case *ast.ImportSpec:
		im := x.Path.Value
		if strings.Contains(im, "google.golang.org/grpc") || im == "\"net\"" {
			c.Delete()
		} else {
			// CHeck if import is our protocols and perform replacements
			lookupPath := strings.Trim(im, "\"")
			fmt.Println("Checking replacement", lookupPath)
			dump.Print(r.protoImportReplacements)
			if v, ok := r.protoImportReplacements[lookupPath]; ok {
				x.Path.Value = fmt.Sprintf("\"%s\"", v)
				c.Replace(x)
				delete(r.protoImportReplacements, lookupPath)
			}
		}
	}

	return true
}

func (r *srcRewriter) Rewrite(file string) (Rewritten, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	f.Name.Name = "impl"

	astutil.Apply(f, nil, r.applyFunction)

	// Add import of stub libraries to file
	newDecls := make([]ast.Decl, len(extraPackages) + len(f.Decls))

	copy(newDecls[len(extraPackages):], f.Decls)

	for n, pkg := range extraPackages {
		newDecls[n] =
			&ast.GenDecl{
				Tok: token.IMPORT,
				Specs: []ast.Spec{
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf("\"%s\"", pkg)}},
				},
			}
	}

	f.Decls = newDecls

	if len(r.protoImportReplacements) > 0 {
		return nil, errors.New("Some expected protocol import packages were not replaced")
	}

	return &srcRewritten{
		fset:     fset,
		ast_file: f,
	}, nil
}

func (r *srcRewritten) Format() (string, error) {
	var writer bytes.Buffer
	printer.Fprint(&writer, r.fset, r.ast_file)
	res := writer.String()
	return res, nil
}

func (r *srcRewritten) Write(file string) error {
	fmtsrc, err := r.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(file, []byte(fmtsrc), 0644)
}
