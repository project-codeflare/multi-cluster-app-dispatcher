module github.com/IBM/multi-cluster-app-dispatcher

go 1.16

require (
	github.com/emicklei/go-restful v2.14.3+incompatible
	github.com/emicklei/go-restful-swagger12 v0.0.0-20201014110547-68ccff494617
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/googleapis/gnostic v0.4.1
	github.com/kubernetes-sigs/custom-metrics-apiserver v0.0.0-20210311094424-0ca2b1909cdc
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	golang.org/x/tools v0.1.1 // indirect
	k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/apiserver v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/component-base v0.20.0
	k8s.io/gengo v0.0.0-20210203185629-de9496dff47b
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/metrics v0.20.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace (
	golang.org/x/tools => golang.org/x/tools v0.1.1 // indirect
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/metrics => k8s.io/metrics v0.20.0
)
