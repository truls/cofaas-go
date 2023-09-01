package main

import (
	"errors"
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "1.3.0"

var requireUnimplemented *bool

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-cofaas-component %v\n", version)
		return
	}

	protogen.Options{
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		if len(gen.Files) > 2 || len(gen.Files) == 0 {
			return errors.New("Specify one or two input files where the first file is the export protocol and the second file is the import protocol")
		}
		exportFile := gen.Files[0]
		var importFile *protogen.File;
		if len(gen.Files) > 1 {
			importFile = gen.Files[1]
		}

		GenerateFile(gen, exportFile, importFile)
		return nil
	})
}
