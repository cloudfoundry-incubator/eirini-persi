module github.com/SUSE/eirini-persi

go 1.14

require (
	code.cloudfoundry.org/cf-operator v1.0.1-0.20200413083459-fb39a29ad746
	github.com/SUSE/eirinix v0.2.1-0.20200420122346-85a6c535b0ad
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pelletier/go-toml v1.3.0 // indirect
	github.com/spf13/cobra v0.0.7
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.6.3
	go.uber.org/zap v1.15.0
	k8s.io/api v0.0.0-20200404061942-2a93acf49b83
	k8s.io/apimachinery v0.0.0-20200410010401-7378bafd8ae2
	k8s.io/client-go v0.0.0-20200330143601-07e69aceacd6
	sigs.k8s.io/controller-runtime v0.4.0
)
