package insecure

func NewCredentials() insecureTC {
	return insecureTC{}
}

type insecureTC struct{}
