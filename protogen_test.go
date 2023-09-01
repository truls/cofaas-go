package cofaas

import (
	"testing"

	opt "github.com/moznion/go-optional"
)

func TestGenGrpcCode(t *testing.T) {
	compareGoldenFile(t, "helloworld.proto", nil, call1test(GenGrpcCode), *update, *verbose)
	//compareGoldenFile(t, "prodcon.proto", GenGrpcCode, *update, *verbose)
}

func TestGenProtoCode(t *testing.T) {
	compareGoldenFile(t, "helloworld_protogen.proto", nil, call1test(GenProtoCode), *update, *verbose)
	compareGoldenFile(t, "prodcon_protogen.proto", nil, call1test(GenProtoCode), *update, *verbose)
}

func TestGenComponentCode(t *testing.T) {
	compareGoldenFile(t, "helloworld_component.proto", opt.Some("prodcon.proto"), GenComponentCode, *update, *verbose)
	//compareGoldenFile(t, "prodcon.proto", GenGrpcCode, *update, *verbose)
}
