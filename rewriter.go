package cofaas

import (
	"errors"
	"strings"

	opt "github.com/moznion/go-optional"
)

type Rewriter interface {
	Rewrite(fileName string) (Rewritten, error)
	AddGrpcReplacement(pkgName string, replacement opt.Option[string], isStdLib bool)
}

type Rewritten interface {
	Format() (string, error)
	Write(fileNmae string) error
}

func GetRewriter(file string, protoImportReplacements map[string]string) (Rewriter, error) {
	var rewriter Rewriter
	if strings.HasSuffix(file, ".mod") {
		rewriter = NewModRewriter()
	} else if strings.HasSuffix(file, ".go") {
		rewriter = NewSrcRewriter(protoImportReplacements)
	} else {
		return nil, errors.New("only supports rewriting .go and .mod files")
	}

	return rewriter, nil
}
