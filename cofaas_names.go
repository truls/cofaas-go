package cofaas

type CofaasName string

const (
	AppNameBase CofaasName = "cofaas/application/"
	ProtoNameBase CofaasName = "cofaas/proto/"
)

func (n CofaasName) Ident(i string) CofaasName {
	return n + CofaasName(i)
}

func (n CofaasName) String() string {
	return string(n)
}

const (
	ComponentName CofaasName = AppNameBase + "component"
	ImplName CofaasName = AppNameBase + "impl"
)
