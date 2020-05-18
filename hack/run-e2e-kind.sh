#!/bin/bash

export ROOT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
export LOG_LEVEL=3
export CLEANUP_CLUSTER=${CLEANUP_CLUSTER:-1}
export CLUSTER_CONTEXT="--name test"
export IMAGE_NGINX="nginx:latest"
export IMAGE_BUSYBOX="busybox:latest"
export KIND_OPT=${KIND_OPT:=" --config ${ROOT_DIR}/hack/e2e-kind-config.yaml"}
export KA_BIN=_output/bin
export WAIT_TIME="20s"
export IMAGE_REPOSITORY_MCAD="${1}"
export IMAGE_TAG_MCAD="${2}"
export MCAD_IMAGE_PULL_POLICY="${3-Always}"
export IMAGE_MCAD="${IMAGE_REPOSITORY_MCAD}:${IMAGE_TAG_MCAD}"

sudo apt-get update && sudo apt-get install -y apt-transport-https
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list
sudo apt-get update
sudo apt-get install -y kubectl

# Download kind binary (0.2.0)
sudo curl -o /usr/local/bin/kind -L https://github.com/kubernetes-sigs/kind/releases/download/v0.5.1/kind-linux-amd64
sudo chmod +x /usr/local/bin/kind

# check if kind installed
function check-prerequisites {
  echo "checking prerequisites"
  which kind >/dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo "kind not installed, exiting."
    exit 1
  else
    echo -n "found kind, version: " && kind version
  fi

  which kubectl >/dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo "kubectl not installed, exiting."
    exit 1
  else
    echo -n "found kubectl, " && kubectl version --short --client
  fi
  
  if [[ $IMAGE_REPOSITORY_MCAD == "" ]]
  then
    echo "No MCAD image was provided."
    exit 1
  elif [[ $IMAGE_TAG_MCAD == "" ]]
  then
    echo "No MCAD image tag was provided for: ${IMAGE_REPOSITORY_MCAD}."
    exit 1
  else
    echo -n "end to end test with ${IMAGE_MCAD}."
  fi
}

function kind-up-cluster {
  check-prerequisites
  echo "Running kind: [kind create cluster ${CLUSTER_CONTEXT} ${KIND_OPT}]"
  kind create cluster ${CLUSTER_CONTEXT} ${KIND_OPT} --wait ${WAIT_TIME}

  docker images
  docker pull ${IMAGE_BUSYBOX}
  docker pull ${IMAGE_NGINX}
  if [[ "$MCAD_IMAGE_PULL_POLICY" = "Always" ]]
  then
    docker pull ${IMAGE_MCAD}
  fi
  docker images
  
  kind load docker-image ${IMAGE_NGINX} ${CLUSTER_CONTEXT}
  kind load docker-image ${IMAGE_BUSYBOX} ${CLUSTER_CONTEXT}
  kind load docker-image ${IMAGE_MCAD} ${CLUSTER_CONTEXT}
}

# clean up
function cleanup {
    echo "==========================>>>>> Cleaning up... <<<<<=========================="
    echo " "

    echo "Custom Resource Definitions..."
    kubectl get crds -A -o yaml
    kubectl describe crds -A

    echo "---"
    echo "AppWrappers..."
    kubectl get appwrappers -A -o yaml
    kubectl describe appwrappers -A

    echo "---"
    echo "'test' Pod list..."
    kubectl get pods -n test

    echo "---"
    echo "'test' Pod yaml..."
    kubectl get pods -n test -o yaml

    echo "---"
    echo "'test' Pod descriptions..."
    kubectl describe pods -n test

    echo "---"
    echo "'all' Namespaces  list..."
    kubectl get namespaces

    echo "---"
    echo "'aw-namespace-1' Namespace  list..."
    kubectl get namespace aw-namespace-1 -o yaml

    echo "===================================================================================="
    echo "==========================>>>>> MCAD Controller Logs <<<<<=========================="
    echo "===================================================================================="
    kubectl logs ${mcad_pod} -n kube-system

    kind delete cluster ${CLUSTER_CONTEXT}
}

deleteme_function() {
cat <<EOF > aw-ss.0.yaml
apiVersion: arbitrator.incubator.k8s.io/v1alpha1
kind: AppWrapper
metadata:
  name: hellodiana-2-test-0
spec:
  schedulingSpec:
    minAvailable: 2
  resources:
    Items:
    - replicas: 1
      type: StatefulSet
      template:
        apiVersion: apps/v1 # for versions before 1.9.0 use apps/v1beta2
        kind: StatefulSet
        metadata:
          name: hellodiana-2-test-0
          labels:
            app: hellodiana-2-test-0
        spec:
          selector:
            matchLabels:
              app: hellodiana-2-test-0
          replicas: 2
          template:
            metadata:
              labels:
                app: hellodiana-2-test-0
                size: "2"
            spec:
              containers:
               - name: hellodiana-2-test-0
                 image: nginx
                 ports:
                 - containerPort: 80
EOF

  echo "---" 
  echo "kubectl get statefulsets"
  kubectl get statefulsets
  
  echo "---" 
  echo "kubectl create -f  aw-ss.0.yaml"
  kubectl create -f  aw-ss.0.yaml

  echo "---" 
  echo "kubectl get statefulsets"
  kubectl get statefulsets

  echo "---" 
  echo "kubectl describe statefulsets"
  kubectl describe statefulsets

  echo "---" 
  echo "kubectl get pods"
  kubectl get pods

  echo "---" 
  echo "kubectl describe pods"
  kubectl describe pods
}

function kube-test-env-up {
    cd ${ROOT_DIR}

    echo "---"
    export KUBECONFIG="$(kind get kubeconfig-path ${CLUSTER_CONTEXT})"

    echo "---"
    echo "KUBECONFIG file: ${KUBECONFIG}"

    echo "---"
    echo "kubectl version"
    kubectl version

    echo "---"
    echo "kubectl config current-context"
    kubectl config current-context

    echo "---"
    echo "kubectl get nodes"
    kubectl get nodes -o wide

    # Show available resources of cluster nodes

    echo "---"
    echo "kubectl describe nodes"
    kubectl describe nodes

    # Hack to setup for 'go test' call which expects this path.
    if [ ! -z $HOME/.kube/config ]
    then
      cp $KUBECONFIG $HOME/.kube/config

      echo "---"
      cat $HOME/.kube/config
    fi

    # Install Helm Client

    echo "---"
    echo "Installing Helm Client..."
    curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get > install-helm.sh
    chmod u+x install-helm.sh
    cat install-helm.sh
    ./install-helm.sh --version v2.16.3

    # Start Helm Server
    echo "Installing Helm Server..."
    kubectl -n kube-system create serviceaccount tiller
    kubectl create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount=kube-system:tiller

    echo "Initialize Helm Server..."
    helm init --service-account tiller
    echo "Wait for Helm Server to complete startup..."
    sleep 25

    echo "Getting Helm Server info..."
    tiller_pod=$(kubectl get pods --namespace kube-system | grep tiller | awk '{print $1}')

    kubectl describe pod ${tiller_pod} -n kube-system

    helm version 

    cd deployment

    # start mcad controller
    echo "Starting MCAD Controller..."
    echo "helm install kube-arbitrator namespace kube-system wait set resources.requests.cpu=1000m set resources.requests.memory=1024Mi set resources.limits.cpu=1000m set resources.limits.memory=1024Mi set image.repository=$IMAGE_REPOSITORY_MCAD set image.tag=$IMAGE_TAG_MCAD set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY debug"
    helm install kube-arbitrator --namespace kube-system --wait --set resources.requests.cpu=1000m --set resources.requests.memory=1024Mi --set resources.limits.cpu=1000m --set resources.limits.memory=1024Mi --set image.repository=$IMAGE_REPOSITORY_MCAD --set image.tag=$IMAGE_TAG_MCAD --set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY --debug

    sleep 10
    helm list
    mcad_pod=$(kubectl get pods -n kube-system | grep xqueuejob | awk '{print $1}')
    if [[ "$mcad_pod" != "" ]]
    then
        kubectl get pod ${mcad_pod} -n kube-system -o yaml
    fi

    deleteme_function
}


trap cleanup EXIT

kind-up-cluster

kube-test-env-up

cd ${ROOT_DIR}

echo "==========================>>>>> Running E2E tests... <<<<<=========================="
go test ./test/e2e -v -timeout 30m
