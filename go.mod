module github.com/IBM/multi-cluster-app-dispatcher

go 1.16

require (
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	k8s.io/api v0.21.0 // indirect
	k8s.io/apimachinery v0.21.0 // indirect
	k8s.io/client-go v0.21.0 // indirect
	k8s.io/metrics v0.21.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.21.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.0
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/code-generator => k8s.io/code-generator v0.21.0
	k8s.io/metrics => k8s.io/metrics v0.21.0
)
