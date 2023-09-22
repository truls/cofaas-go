package cofaas

import (
	"flag"
	"os"
	"testing"

	opt "github.com/moznion/go-optional"
)

var (
	update  = flag.Bool("update", false, "update the golden files of this test")
	verbose = flag.Bool("verbose", false, "be verbose when test fails")
)

func rewrite(file string, r Rewriter) (string, error) {

	res, err := r.Rewrite(file)
	if err != nil {
		return "", err
	}

	return res.Format()
}

func testRewriter(t *testing.T, f string) {
	protoReplacements := map[string]string{
		"cofaas_orig/protos/helloworld": "cofaas/protos/helloworld",
		"cofaas_orig/protos/prodcon":    "cofaas/protos/prodcon",
		"google.golang.org/grpc": "github.com/truls/cofaas-go/stubs/grpc",
		"google.golang.org/grpc/reflection": "github.com/truls/cofaas-go/stubs/grpc/reflection",
		"google.golang.org/grpc/credentials/insecure": "github.com/truls/cofaas-go/stubs/grpc/credentials/insecure",
		"net": "github.com/truls/cofaas-go/stubs/net",
	}

	r, err := GetRewriter(f, protoReplacements)
	if err != nil {
		t.Error(err)
	}

	compareGoldenFile(t, f, nil, func(file string, a2 opt.Option[string]) (string, error) {
		return rewrite(file, r)
	}, *update, *verbose)

}

func TestRewriteModule(t *testing.T) {
	testRewriter(t, "go.mod")
}

func TestRewriteFile(t *testing.T) {
	testRewriter(t, "producer.go")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
