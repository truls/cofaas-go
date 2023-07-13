package cofaas

import (
	"testing"
)

func TestGenGrpcCode(t *testing.T) {
	compareGoldenFile(t, "helloworld.proto", GenGrpcCode, *update, *verbose)
	//compareGoldenFile(t, "prodcon.proto", GenGrpcCode, *update, *verbose)
}

func TestGenProtoCode(t *testing.T) {
	compareGoldenFile(t, "helloworld_protogen.proto", GenProtoCode, *update, *verbose)
	compareGoldenFile(t, "prodcon_protogen.proto", GenProtoCode, *update, *verbose)
}
