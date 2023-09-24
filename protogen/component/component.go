package main

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	contextPackage = protogen.GoImportPath("context")
	errorsPackage  = protogen.GoImportPath("errors")
	fmtPackage     = protogen.GoImportPath("fmt")
	implPackage    = protogen.GoImportPath("cofaas/application/impl")
)

// FileDescriptorProto.package field number
const fileDescriptorProtoPackageFieldNumber = 2

// FileDescriptorProto.syntax field number
const fileDescriptorProtoSyntaxFieldNumber = 12

// generateFile generates a _grpc.pb.go file containing gRPC service definitions.
func GenerateFile(gen *protogen.Plugin, exportFile *protogen.File, importFile *protogen.File) *protogen.GeneratedFile {
	if len(exportFile.Services) == 0 {
		return nil
	}
	filename := "component.go"
	g := gen.NewGeneratedFile(filename, exportFile.GoImportPath)
	// Attach all comments associated with the syntax field.
	//genLeadingComments(g, file.Desc.SourceLocations().ByPath(protoreflect.SourcePath{fileDescriptorProtoSyntaxFieldNumber}))
	// g.P("// Code generated by protoc-gen-cofaas-go-grpc. DO NOT EDIT.")
	// g.P("// versions:")
	// g.P("// - protoc-gen-cofaas-go-grpc v", version)
	// g.P("// - protoc             ", protocVersion(gen))
	// if file.Proto.GetOptions().GetDeprecated() {
	// 	g.P("// ", file.Desc.Path(), " is a deprecated file.")
	// } else {
	// 	g.P("// source: ", file.Desc.Path())
	// }
	// g.P()
	// Attach all comments associated with the package field.
	//genLeadingComments(g, file.Desc.SourceLocations().ByPath(protoreflect.SourcePath{fileDescriptorProtoPackageFieldNumber}))
	g.P("package main")
	g.P()
	generateFileContent(gen, exportFile, importFile, g)
	return g
}

func generateFileContent(gen *protogen.Plugin, exportFile *protogen.File, importFile *protogen.File, g *protogen.GeneratedFile) {
	genExportStructDecl(exportFile, g)
	genImportStructDecl(importFile, g)
	g.P()

	genInitFunc(gen, exportFile, importFile, g)
	g.P()

	// Generate handlers for export functions
	genInitComponent(gen, exportFile, importFile, g)
	g.P()

	// Generate handlers for import functions

	genExportHandlers(gen, exportFile, g)
	g.P()

	genImportHandlers(gen, importFile, g)
	g.P()

	g.P("//go:generate wit-bindgen tiny-go ../../wit --world producer-interface --out-dir=gen")
	g.P("func main() {}")

}

func genExportStructName(exportFile *protogen.File) string {
	return *exportFile.Proto.Package + "Impl"
}

func genImportStructName(importFile *protogen.File) string {
	return *importFile.Proto.Package + "ClientImpl"
}

func genExportStructDecl(exportFile *protogen.File, g *protogen.GeneratedFile) {
	g.P("type " + genExportStructName(exportFile) + " struct{}")
}

func genImportStructDecl(importFile *protogen.File, g *protogen.GeneratedFile) {
	if importFile != nil {
		g.P("type " + genImportStructName(importFile) + " struct{}")
	}
}

func getInterfaceIdent(ident string, g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       ident,
		GoImportPath: "cofaas/application/component/gen",
	})
}

func getProtoIdent(ident string, importFile *protogen.File, g *protogen.GeneratedFile) string {
	var base protogen.GoImportPath = "cofaas/proto/"
	var name = *(*protogen.GoImportPath)(unsafe.Pointer(&importFile.GoPackageName))
	pkgName := base + name
	return g.QualifiedGoIdent(pkgName.Ident(ident))
}

// getService asserts that file only defines a single service and
// returns that service
func getService(gen *protogen.Plugin, file *protogen.File) *protogen.Service {
	svcs := file.Services
	if len(svcs) != 1 {
		gen.Error(errors.New("protocol must define a single service"))
	}
	return svcs[0]
}

func genInitFunc(gen *protogen.Plugin, exportFile *protogen.File, importFile *protogen.File, g *protogen.GeneratedFile) {
	g.P("func init() {")
	// g.("github.com/truls/chained-service-example/producer/component/gen")
	g.P("a := " + genExportStructName(exportFile) + "{}")
	g.P(getInterfaceIdent("SetExportsCofaasApplication"+getService(gen, exportFile).GoName, g) + "(a)")
	g.P()

	if importFile != nil {
		g.P("c := " + genImportStructName(importFile) + "{}")
		g.P(getProtoIdent("Set"+getService(gen, importFile).GoName+"ClientImplementation", importFile, g) + "(c)")
	}

	g.P("}")
}

func genInitComponent(gen *protogen.Plugin, exportFile *protogen.File, importFile *protogen.File, g *protogen.GeneratedFile) {
	g.P("func (" + genExportStructName(exportFile) + ") InitComponent() {")
	g.P(g.QualifiedGoIdent(implPackage.Ident("Main")) + "()")
	if importFile != nil {
		g.P(getInterfaceIdent("CofaasApplication"+getService(gen, importFile).GoName+"InitComponent", g) + "()")
	}
	g.P("}")
}

func genExportHandlers(gen *protogen.Plugin, exportFile *protogen.File, g *protogen.GeneratedFile) {
	svc := getService(gen, exportFile)
	for _, m := range svc.Methods {
		genExportMethod(exportFile, m, g)
	}
}

func genExportMethod(exportFile *protogen.File, method *protogen.Method, g *protogen.GeneratedFile) {
	outputName := getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.Output.GoIdent.GoName, g)
	retType := getInterfaceIdent("Result", g) + "[" + outputName + ", int32]"

	g.P("func (" + genExportStructName(exportFile) + ") " +
		method.GoName +
		" (arg " + getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.Input.GoIdent.GoName, g) + ") " + retType + "{")
	g.P("param := " + getProtoIdent(method.Input.GoIdent.GoName, exportFile, g) + "{" + genParamMap(method.Input, "arg") + "}")
	g.P("res, err := " + getProtoIdent("ServerImplementation."+method.GoName, exportFile, g) + "(context.TODO(), &param)")
	g.P("if err != nil {")
	g.P("return " + retType + "{Kind: " + getInterfaceIdent("Err", g) + ", Err: 1, Val: " + getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.Output.GoIdent.GoName, g) + "{}}")
	g.P("}")
	g.P()
	g.P("return " + retType + "{Kind: " + getInterfaceIdent("Ok", g) + ", Err: 0, Val: " + getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.Output.GoIdent.GoName, g) + "{" + genParamMap(method.Output, "res") + "}}")
	g.P("}")
}

func genImportHandlers(gen *protogen.Plugin, importFile *protogen.File, g *protogen.GeneratedFile) {
	svc := getService(gen, importFile)
	for _, m := range svc.Methods {
		genImportMethod(importFile, m, g)
	}
}

func genImportMethod(importFile *protogen.File, method *protogen.Method, g *protogen.GeneratedFile) {
	g.P("func (" + genImportStructName(importFile) + ") " + method.GoName + "(ctx " + g.QualifiedGoIdent(contextPackage.Ident("Context")) + ", in *" + getProtoIdent(method.Input.GoIdent.GoName, importFile, g) + ", opts ...interface{}) (*" + getProtoIdent(method.Output.GoIdent.GoName, importFile, g) + ", error) {")
	g.P("param := " +
		getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.Input.GoIdent.GoName, g) + "{" + genParamMap(method.Input, "in") + "}")
	g.P("res := " + getInterfaceIdent("CofaasApplication"+method.Parent.GoName+method.GoName, g) + "(param)")
	g.P("if res.IsErr() {")
	g.P("return nil, " + g.QualifiedGoIdent(fmtPackage.Ident("Errorf")) + `("Call ` + method.GoName + ` failed with code: %s", res.Unwrap())`)
	g.P("}")
	g.P("resu := res.Unwrap()")
	g.P("return &" + getProtoIdent(method.Output.GoIdent.GoName, importFile, g) + "{" + genParamMap(method.Output, "resu") + "}, nil")
	g.P("}")
}

func genParamMap(msg *protogen.Message, moduleName string) string {
	res := strings.Builder{}
	for _, f := range msg.Fields {
		res.WriteString(f.GoName)
		res.WriteString(": " + moduleName + ".")
		res.WriteString(f.GoName + ",")
	}
	return res.String()
}

func genLeadingComments(g *protogen.GeneratedFile, loc protoreflect.SourceLocation) {
	for _, s := range loc.LeadingDetachedComments {
		g.P(protogen.Comments(s))
		g.P()
	}
	if s := loc.LeadingComments; s != "" {
		g.P(protogen.Comments(s))
		g.P()
	}
}

func protocVersion(gen *protogen.Plugin) string {
	v := gen.Request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}
	var suffix string
	if s := v.GetSuffix(); s != "" {
		suffix = "-" + s
	}
	return fmt.Sprintf("v%d.%d.%d%s", v.GetMajor(), v.GetMinor(), v.GetPatch(), suffix)
}
