package cofaas

import (
	opt "github.com/moznion/go-optional"
	"golang.org/x/mod/modfile"
	"os"
)

type importReplacementMap = map[string]*importReplacement

type modRewriter struct {
	Rewriter
	// Will be true if a file was pared
	parsed *modfile.File
	// Map of packages that should be removed from require and replace
	// sections
	requireHits importReplacementMap
}

type modRewritten struct {
	Rewritten
	// Will be true if a file was pared
	parsed *modfile.File
}

type importReplacement struct {
	seen     bool
	isStdLib bool
	require  opt.Option[string]
}

func newImportReplacement(require opt.Option[string], isStdLib bool) *importReplacement {
	return &importReplacement{
		seen:     false,
		isStdLib: isStdLib,
		require:  require,
	}
}

func NewModRewriter() Rewriter {
	return &modRewriter{
		requireHits: importReplacementMap{
			"google.golang.org/grpc": newImportReplacement(opt.Some("github.com/truls/cofaas-go/stubs/grpc"),
				false),
			"google.golang.org/protobuf": newImportReplacement(nil, false),
		},
	}
}

func (r *modRewriter) AddGrpcReplacement(pkgName string, replacement opt.Option[string], isStdLib bool) {
	if _, ok := r.requireHits[pkgName]; !ok {
		r.requireHits[pkgName] = newImportReplacement(replacement, isStdLib)
	}
}

func (r *modRewriter) doRewrites() {
	f := r.parsed

	// Remove grpc and protobuf imports from require
	for _, req := range f.Require {
		includeName := req.Mod.Path
		if _, ok := r.requireHits[includeName]; ok {
			r.parsed.DropRequire(includeName)
			r.requireHits[includeName].seen = true
		}
	}

	// Filter replace statements
	for _, rep := range f.Replace {
		old := rep.Old.Path
		if _, ok := r.requireHits[old]; ok {
			r.parsed.DropReplace(old, "")
		}
	}
}

func (r *modRewriter) Rewrite(file string) (Rewritten, error) {
	contents, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	res, err := modfile.Parse(file, contents, nil)
	if err != nil {
		return nil, err
	}
	r.parsed = res

	r.doRewrites()

	return &modRewritten{
		parsed: res,
	}, nil

}

func (r *modRewritten) Format() (string, error) {
	res, err := r.parsed.Format()
	if err != nil {
		return "", err
	}
	return string(res), nil

}

func (r *modRewritten) Write(file string) error {

	formatted, err := r.Format()
	if err != nil {
		return err
	}

	return os.WriteFile(file, []byte(formatted), 0644)
}
