package cofaas

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"

	"golang.org/x/tools/go/ast/astutil"
)

// https://eli.thegreenplace.net/2021/rewriting-go-source-code-with-ast-tooling/

type srcRewriter struct {
	Rewriter
}

type srcRewritten struct {
	Rewritten
	fset     *token.FileSet
	ast_file *ast.File
}

func NewSrcRewriter() Rewriter {
	return &srcRewriter{}
}

func applyFunction(c *astutil.Cursor) bool {
	n := c.Node()

	switch x := n.(type) {
	case *ast.FuncDecl:
		id := x.Name
		// Export main function
		if id.Name == "main" {
			x.Name.Name = "Main"
			c.Replace(x)
		}
	case *ast.Package:
		// Rename main package
		if x.Name == "main" {
			x.Name = "cofaas_main"
			c.Replace(x)
		}
	case *ast.ImportSpec:
		c.Replace(x)
	}

	return true
}

func (r *srcRewriter) Rewrite(file string) (Rewritten, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	astutil.Apply(f, nil, applyFunction)

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
