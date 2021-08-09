module github.com/IBM/multi-cluster-app-dispatcher

go 1.16

require (
	github.com/emicklei/go-restful v2.14.3+incompatible
	github.com/emicklei/go-restful-swagger12 v0.0.0-20201014110547-68ccff494617
	github.com/googleapis/gnostic v0.4.1
	github.com/kubernetes-sigs/custom-metrics-apiserver v0.0.0-20210311094424-0ca2b1909cdc
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.ibm.com/ai-foundation/quota-manager v0.0.0-20210624161340-4090ece12dab
	golang.org/x/tools v0.1.1 // indirect
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery v0.20.5
	k8s.io/apiserver v0.20.5
	k8s.io/client-go v0.20.5
	k8s.io/component-base v0.20.5
	k8s.io/gengo v0.0.0-20210203185629-de9496dff47b
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/metrics v0.20.5
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/scheduler-plugins v0.0.0
)

replace (
	golang.org/x/tools => golang.org/x/tools v0.1.1 // indirect
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.0 // indirect
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/apiserver => k8s.io/apiserver v0.20.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.5 // indirect
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.5 // indirect
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.5 // indirect
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.5 // indirect
	k8s.io/controller-manager => k8s.io/controller-manager v0.20.5 // indirect
	k8s.io/cri-api => k8s.io/cri-api v0.20.5 // indirect
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.5 // indirect
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.5 // indirect
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.5 // indirect
	k8s.io/kubectl => k8s.io/kubectl v0.20.5 // indirect
	k8s.io/kubelet => k8s.io/kubelet v0.20.5 // indirect
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.5 // indirect
	k8s.io/metrics => k8s.io/metrics v0.20.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.5
	sigs.k8s.io/scheduler-plugins => github.ibm.com/platformcomputing/k8s-plugins v0.0.0-20210729025643-afe9bf1a6d8e
)
