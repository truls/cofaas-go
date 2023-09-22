package metadata

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	opt "github.com/moznion/go-optional"
)

func TestParse(t *testing.T) {
	res, err := Parse("testdata/test.yaml", false)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	expected := Metadata{
		ExportProto: &ProtoSpec{
			Import: string("cofaas_orig/protos/helloworld"),
			Name:   string("helloworld"),
			Path:   string("../../protos/helloworld.proto"),
		},
		ImportProto: opt.Some(&ProtoSpec{
			Import: string("cofaas_orig/protos/prodcon"),
			Name:   string("prodcon"),
			Path:   string("../../protos/prodcon.proto"),
		}),
	}

	if diff := cmp.Diff(*res, expected); diff != "" {
		t.Fatalf("Expected and actual results differ\n%s", diff)
	}
}
