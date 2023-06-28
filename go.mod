module github.com/truls/cofaas-go

go 1.20

require (
	github.com/moznion/go-optional v0.10.0
	golang.org/x/mod v0.11.0
	golang.org/x/tools v0.1.12
)

require github.com/sergi/go-diff v1.3.1

replace github.com/truls/cofaas-go/stubs/grpc v0.0.0 => ./stubs/grpc

replace github.com/truls/cofaas-go/stubs/net v0.0.0 => ./stubs/net
