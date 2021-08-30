#!/bin/bash
# Copyright 2014 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

export ROOT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
export LOG_LEVEL=3
export CLEANUP_CLUSTER=${CLEANUP_CLUSTER:-1}
export CLUSTER_CONTEXT="--name test"
# Using older image due to older version of kubernetes cluster"
export IMAGE_NGINX="nginx:1.15.12"
export IMAGE_ECHOSERVER="k8s.gcr.io/echoserver:1.4"
export KIND_OPT=${KIND_OPT:=" --config ${ROOT_DIR}/hack/e2e-kind-config.yaml"}
export KA_BIN=_output/bin
export WAIT_TIME="20s"
export IMAGE_REPOSITORY_MCAD="${1}"
export IMAGE_TAG_MCAD="${2}"
export MCAD_IMAGE_PULL_POLICY="${3}"
export BUILD_TAG_MCAD="${4}"
export IMAGE_MCAD="${IMAGE_REPOSITORY_MCAD}:${IMAGE_TAG_MCAD}"
export KUBECTL_VERSION=1.20.0-00

######
# Vars
######
return_code=0

##################
# Install prereqs.
##################

# Verify paramenter
if [ "$#" -lt "3" ]
then
    echo "Illegal number of parameters, requires 3."
    echo " "
    exit 2
fi

sudo apt-get update && sudo apt-get install -y apt-transport-https
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list
sudo apt-get update

# Using kubernetes cluster client CLI"
sudo apt-get install -y kubectl=$KUBECTL_VERSION

# Download kind binary
sudo curl -o /usr/local/bin/kind -L https://github.com/kubernetes-sigs/kind/releases/download/v0.11.0/kind-linux-amd64
sudo chmod +x /usr/local/bin/kind

# Download krew
(
  set -x; cd "$(mktemp -d)" &&
  OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
  ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
  curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/krew.tar.gz" &&
  tar zxvf krew.tar.gz &&
  KREW=./krew-"${OS}_${ARCH}" &&
  "$KREW" install krew
)
export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$PATH"

# Install kuttl
kubectl krew install kuttl && true

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

  kuttl_plugin=$(kubectl plugin list 2>&1 | grep kuttl)
  if [[ "$kuttl_plugin" == "" ]]; then
    echo "kubectl kuttl not installed, exiting."
    exit 1
  else
    echo -n "found kubectl kuttl, " && kubectl plugin list
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
  docker pull ${IMAGE_ECHOSERVER}
  docker pull ${IMAGE_NGINX}
  if [[ "$MCAD_IMAGE_PULL_POLICY" = "Always" ]]
  then
    docker pull ${IMAGE_MCAD}
  fi
  docker images
  
  kind load docker-image ${IMAGE_NGINX} ${CLUSTER_CONTEXT}
  kind load docker-image ${IMAGE_ECHOSERVER} ${CLUSTER_CONTEXT}
  kind load docker-image ${IMAGE_MCAD} ${CLUSTER_CONTEXT}
}

# clean up
function cleanup {
    echo "==========================>>>>> Cleaning up... <<<<<=========================="
    echo " "


    echo "Custom Resource Definitions..."
    echo "kubectl get crds"
    kubectl get crds

    echo "---"
    echo "Get All AppWrappers..."
    kubectl get appwrappers --all-namespaces -o yaml

    echo "---"
    echo "Describe all AppWrappers..."
    kubectl describe appwrappers --all-namespaces

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
    echo "kubectl logs ${mcad_pod} -n kube-system"
    kubectl logs ${mcad_pod} -n kube-system

    kind delete cluster ${CLUSTER_CONTEXT}
}

debug_function() {
  echo "---"
  echo "kubectl create namespace test"
  kubectl create namespace test

cat <<EOF > aw-ss.0.yaml
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: hellodiana-2-test-0
  namespace: test
spec:
  schedulingSpec:
    minAvailable: 2
  resources:
    Items:
    - replicas: 1
      metadata:
        name: hellodiana-2-test-0
        namespace: test
      type: StatefulSet
      template:
        apiVersion: apps/v1 # for versions before 1.9.0 use apps/v1beta2
        kind: StatefulSet
        metadata:
          name: hellodiana-2-test-0
          namespace: test
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
                 image: k8s.gcr.io/echoserver:1.4
                 imagePullPolicy: Always
                 ports:
                 - containerPort: 80
EOF

  echo "---" 
  echo "kubectl get statefulsets"
  kubectl get statefulsets -n test
  
  echo "---" 
  echo "kubectl create -f  aw-ss.0.yaml"
  kubectl create -f  aw-ss.0.yaml

  echo "---"
  echo "kubectl get appwrappers -o yaml"
  kubectl get appwrappers -o yaml -n test

  sleep 5

  echo "---" 
  echo "kubectl get statefulsets"
  kubectl get statefulsets -n test

  sleep 5

  echo "---" 
  echo "kubectl describe statefulsets"
  kubectl describe statefulsets -n test

  sleep 5

  echo "---" 
  echo "kubectl get pods"
  kubectl get pods -n test

  sleep 5

  echo "---"
  echo "kubectl get pods -o yaml"
  kubectl get pods -n test -o yaml

  echo "---" 
  echo "kubectl describe pods"
  kubectl describe pods -n test

  echo "---"
  echo "kubectl delete -f  aw-ss.0.yaml"
  kubectl delete -f  aw-ss.0.yaml

  echo "---"
  echo "kubectl delete namespace test"
  kubectl delete namespace test
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
    ./install-helm.sh --version v2.17.0

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
    echo "helm install mcad-controller namespace kube-system wait set loglevel=2 set resources.requests.cpu=1000m set resources.requests.memory=1024Mi set resources.limits.cpu=4000m set resources.limits.memory=4096Mi set image.repository=$IMAGE_REPOSITORY_MCAD set image.tag=$IMAGE_TAG_MCAD set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY"
    helm install mcad-controller --namespace kube-system --wait --set loglevel=2 --set resources.requests.cpu=1000m --set resources.requests.memory=1024Mi --set resources.limits.cpu=4000m --set resources.limits.memory=4096Mi --set image.repository=$IMAGE_REPOSITORY_MCAD --set image.tag=$IMAGE_TAG_MCAD --set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY

    sleep 10
    echo "Listing MCAD Controller Helm Chart and Pod YAML..."
    helm list
    mcad_pod=$(kubectl get pods -n kube-system | grep mcad-controller | awk '{print $1}')
    if [[ "$mcad_pod" != "" ]]
    then
        kubectl get pod ${mcad_pod} -n kube-system -o yaml
    fi


    sleep 10
    echo "Listing MCAD Controller Helm Chart and Pod YAML..."
    helm list
    mcad_pod=$(kubectl get pods -n kube-system | grep mcad-controller | awk '{print $1}')
    if [[ "$mcad_pod" != "" ]]
    then
        kubectl get pod ${mcad_pod} -n kube-system -o yaml
    fi

    # Turn off master taints
    kubectl taint nodes --all node-role.kubernetes.io/master-


    # This is meant to orchestrate initial cluster configuration such that accounting tests can be consistent
    echo "---"
    echo "Orchestrate cluster..."
    echo "kubectl cordon test-worker"
    kubectl cordon test-worker
    a=$(kubectl -n kube-system get pods | grep coredns | cut -d' ' -f1)
    for b in $a
    do
      echo "kubectl -n kube-system delete pod $b"
      kubectl -n kube-system delete pod $b
    done
    echo "kubectl uncordon test-worker"
    kubectl uncordon test-worker

    # Show available resources of cluster nodes
    echo "---"
    echo "kubectl describe nodes"
    kubectl describe nodes
}

kuttl_e2e_tests() {
  if [[ "$BUILD_TAG_MCAD" == "" ]]
  then
    kubectl kuttl test
    return_code=$?
  fi
}

trap cleanup EXIT

kind-up-cluster

kube-test-env-up

cd ${ROOT_DIR}

echo "==========================>>>>> Running E2E tests... <<<<<=========================="
kuttl_e2e_tests
if [[ "$return_code" != "0" ]]
then
  exit $return_code
fi

go test ./test/e2e -v -timeout 30m
