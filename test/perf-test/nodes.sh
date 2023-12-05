#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

function help() {
    echo "usage: nodes.sh [-h]"
    echo
    echo "Description: Creates fake KWOK nodes for performance testing"
    echo
    echo "Preconditions: "
    echo "   - The script assumes you've logged into your cluster already. If not, it will tell you to login."
    echo "   - The script checks that you have the kwok-controller installed, otherwise it'll tell you to install it first."
    echo
    echo "Options:"
    echo "  -h       Print this help message"
    echo
}

function check_kubectl_login_status() {
    set +e
    kubectl get ns default &> /dev/null
    res="$?"
    set -e
    OCP="$res"
      if [ $OCP == 1 ]
      then
        echo "You need to login to your Kubernetes Cluster"
        exit 1
      else
        echo
        echo "Nice, looks like you're logged in"
        echo ""
      fi
}

function check_kwok_installed_status() {
    set +e
    kubectl get pod -A |grep kwok-controller &> /dev/null
    res2="$?"
    set -e
    KWOK="$res2"
      if [[ $KWOK == 1 ]]
      then
        echo "You need Install the KWOK Controller first before running this script"
        exit 1
      else
        echo "Nice, the KWOK Controller is installed"
    fi
}

while getopts hf: option; do
    case $option in
        h)
            help
            exit 0
            ;;
        *)
            ;;
    esac
done
shift $((OPTIND-1))

# Track whether we have a valid kubectl login
echo "Checking whether we have a valid cluster login or not..."
check_kubectl_login_status

# Track whether you have the KWOK controller installed
echo "Checking KWOK Controller installation status"
echo
check_kwok_installed_status

echo
read -p "How many simulated KWOK nodes do you want?" NODES

echo "Nodes number is $NODES"
echo " "

COUNTER=1
while [ $COUNTER -le $NODES ]
do
    ORIG_COUNTER=$(($COUNTER - 1))
    echo "Submitting node $COUNTER"
# Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/kwok-node-$ORIG_COUNTER/kwok-node-$COUNTER/g" ${SCRIPT_DIR}/node.yaml ;;
      darwin*)
        sed -i '' "s/kwok-node-$ORIG_COUNTER/kwok-node-$COUNTER/g" ${SCRIPT_DIR}/node.yaml ${SCRIPT_DIR}/node.yaml ;;
      *)
        sed -i "/kwok-node-$ORIG_COUNTER/kwok-node-$COUNTER/g" ${SCRIPT_DIR}/node.yaml ;;
    esac
    kubectl apply -f ${SCRIPT_DIR}/node.yaml
COUNTER=$[$COUNTER +1]
done

    # Let's reset the original node.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/kwok-node-$NODES/kwok-node-0/g" ${SCRIPT_DIR}/node.yaml ;;
      darwin*) 
        sed -i '' "s/kwok-node-$NODES/kwok-node-0/g" ${SCRIPT_DIR}/node.yaml ;;
      *) 
        sed -i "s/kwok-node-$NODES/kwok-node-0/g" ${SCRIPT_DIR}/node.yaml ;;
    esac

# Check for all nodes to report complete
echo "Waiting until all the simualted pods become ready:"
kubectl wait --for=condition=Ready nodes --selector type=kwok --timeout=600s
echo " "
echo "Total amount of simulated nodes requested is: $NODES"
echo "Total number of created nodes is: "`kubectl get nodes --selector type=kwok -o name |wc -l`
kubectl get nodes --selector type=kwok

echo " "
echo "FYI, to clean up the kwow nodes, issue this:"
echo "kubectl get nodes --selector type=kwok -o name | xargs kubectl delete"
