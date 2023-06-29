package cofaas

import (
	"testing"
)

func TestGenProtoCode(t *testing.T) {
	compareGoldenFile(t, "helloworld.proto", GenProtoCode, *update, *verbose)
}
