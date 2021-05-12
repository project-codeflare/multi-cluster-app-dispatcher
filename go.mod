module github.com/IBM/multi-cluster-app-dispatcher

go 1.16

require (
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/googleapis/gnostic v0.4.1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/apiserver v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/component-base v0.21.0
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027
	k8s.io/metrics v0.21.0
)

replace (
	k8s.io/api => k8s.io/api v0.21.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.0
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/code-generator => k8s.io/code-generator v0.21.0
	k8s.io/metrics => k8s.io/metrics v0.21.0
)
