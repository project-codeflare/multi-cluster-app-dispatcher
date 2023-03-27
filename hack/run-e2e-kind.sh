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

export ROOT_DIR="$(dirname "$(dirname "$(readlink -fm "$0")")")"
export LOG_LEVEL=3
export CLEANUP_CLUSTER=${CLEANUP_CLUSTER:-"true"}
export CLUSTER_CONTEXT="--name test"
# Using older image due to older version of kubernetes cluster"
export IMAGE_ECHOSERVER="kicbase/echo-server:1.0"
export KIND_OPT=${KIND_OPT:=" --config ${ROOT_DIR}/hack/e2e-kind-config.yaml"}
export KA_BIN=_output/bin
export WAIT_TIME="20s"
export IMAGE_REPOSITORY_MCAD="${1}"
export IMAGE_TAG_MCAD="${2}"
export MCAD_IMAGE_PULL_POLICY="${3-Always}"
export IMAGE_MCAD="${IMAGE_REPOSITORY_MCAD}:${IMAGE_TAG_MCAD}"
CLUSTER_STARTED="false"

function update_test_host {
  sudo apt-get update && sudo apt-get install -y apt-transport-https curl 
  curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
  echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list
  sudo apt-get update
  # Using older version due to older version of kubernetes cluster"
  sudo apt-get install -y --allow-unauthenticated kubectl=1.17.0-00

  # Download kind binary (0.6.1)
  sudo curl -o /usr/local/bin/kind -L https://github.com/kubernetes-sigs/kind/releases/download/v0.11.0/kind-linux-amd64
  sudo chmod +x /usr/local/bin/kind

  # Installing helm3
  curl -fsSL -o ${ROOT_DIR}/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
  chmod 700 ${ROOT_DIR}/get_helm.sh
  ${ROOT_DIR}/get_helm.sh
  sleep 10
}

# check if pre-requizites are installed.
function check-prerequisites {
  echo "checking prerequisites"
  which kind >/dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "kind not installed, exiting."
    exit 1
  else
    echo -n "found kind, version: " && kind version
  fi

  which kubectl >/dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "kubectl not installed, exiting."
    exit 1
  else
    echo -n "found kubectl, " && kubectl version 
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
    echo "end to end test with ${IMAGE_MCAD}."
  fi
  
  which helm >/dev/null 2>&1
  if [ $? -ne 0 ]
  then
    echo "helm not installed, exiting."
    exit 1
  else
    echo -n "found helm, " && helm version --short
  fi  

}

function kind-up-cluster {
  echo "Running kind: [kind create cluster ${CLUSTER_CONTEXT} ${KIND_OPT}]"
  kind create cluster ${CLUSTER_CONTEXT} ${KIND_OPT} --wait ${WAIT_TIME}
  if [ $? -ne 0 ]
  then
    echo "Failed to start kind cluster"
    exit 1
  fi
  CLUSTER_STARTED="true"

  docker pull ${IMAGE_ECHOSERVER}
  if [[ "$MCAD_IMAGE_PULL_POLICY" = "Always" ]]
  then
    docker pull ${IMAGE_MCAD}
  fi
  docker images
    
  kind load docker-image ${IMAGE_ECHOSERVER} ${CLUSTER_CONTEXT}
  if [ $? -ne 0 ]
  then
    echo "Failed to load image ${IMAGE_ECHOSERVER} in cluster"
    exit 1
  fi 
  
  kind load docker-image ${IMAGE_MCAD} ${CLUSTER_CONTEXT}
  if [ $? -ne 0 ]
  then
    echo "Failed to load image ${IMAGE_MCAD} in cluster"
    exit 1
  fi
}

# clean up
function cleanup {
    echo "==========================>>>>> Cleaning up... <<<<<=========================="
    echo " "
    if [[ ${CLUSTER_STARTED} == "false" ]]
    then
      echo "Cluster was not started, nothing more to do."
      return
    fi  

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
    local mcad_pod=$(kubectl get pods -n kube-system | grep mcad-controller | awk '{print $1}')
    if [[ "$mcad_pod" != "" ]]
    then
      echo "kubectl logs ${mcad_pod} -n kube-system"
      kubectl logs ${mcad_pod} -n kube-system
    fi
    rm -rf ${ROOT_DIR}/get_helm.sh
    if [[ $CLEANUP_CLUSTER == "true" ]]
    then
      kind delete cluster ${CLUSTER_CONTEXT}     
    else 
      echo "Cluster requested to stay up, not deleting cluster"     
    fi 
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
                 image: ${IMAGE_ECHOSERVER}
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
  
    echo "Installing Podgroup CRD"

    kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/scheduler-plugins/277b6bdec18f8a9e9ccd1bfeaf4b66495bfc6f92/config/crd/bases/scheduling.sigs.k8s.io_podgroups.yaml

    # start mcad controller
    echo "Starting MCAD Controller..."
    echo "helm install mcad-controller namespace kube-system wait set loglevel=2 set resources.requests.cpu=1000m set resources.requests.memory=1024Mi set resources.limits.cpu=4000m set resources.limits.memory=4096Mi set image.repository=$IMAGE_REPOSITORY_MCAD set image.tag=$IMAGE_TAG_MCAD set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY"
    helm upgrade --install mcad-controller ${ROOT_DIR}/deployment/mcad-controller  --namespace kube-system --wait --set loglevel=2 --set resources.requests.cpu=1000m --set resources.requests.memory=1024Mi --set resources.limits.cpu=4000m --set resources.limits.memory=4096Mi --set configMap.name=mcad-controller-configmap --set configMap.podCreationTimeout='"120000"' --set configMap.quotaEnabled='"false"' --set coscheduler.rbac.apiGroup=scheduling.sigs.k8s.io --set coscheduler.rbac.resource=podgroups --set image.repository=$IMAGE_REPOSITORY_MCAD --set image.tag=$IMAGE_TAG_MCAD --set image.pullPolicy=$MCAD_IMAGE_PULL_POLICY

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


trap cleanup EXIT

#Only run this function if we are running on the test build machinbe,
#currently  ubuntu 16.04 xenial
if [ "$(lsb_release -c -s 2>&1 | grep xenial)" == "xenial" ]; then 
   update_test_host
fi

check-prerequisites 

kind-up-cluster

kube-test-env-up

echo "==========================>>>>> Running E2E tests... <<<<<=========================="
go test ./test/e2e -v -timeout 55m