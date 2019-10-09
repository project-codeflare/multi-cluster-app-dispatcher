#!/bin/bash

# Licensed Materials - Property of IBM
# (c) Copyright IBM Corporation 2018, 2019. All Rights Reserved.
# Note to U.S. Government Users Restricted Rights:
# Use, duplication or disclosure restricted by GSA ADP Schedule
# Contract with IBM Corp.

###############
# configuration
###############
deployment_name=$1
config=$2
namespace=$3
certPath=$4
frequency=$5
k8s_api_ip_port=$6


######
# vars
######
available_cpu="0"
available_memory="0"

echo -e "
------ Introduction phase for the user-----

this script takes 6 input parameters, e.g.:

Usage: ${0##*/} ~/.kube/conf default 5 

- parameter 1, XQueueJob deployment name, the default XQueueJob
- parameter 2, is the KUBECONFIG file path: ~/.kube/conf
- parameter 3, is the namespace where the CRD instances are created, the default namespace is `default`.
- parameter 4, is the path to the folder of the TLS certs to authenticate with k8s api server 
- parameter 5, is the frequency at which we check over for CRD created, the default is 10 seconds
- parameter 6, is the ip:port of the k8s api server, the default is 192.168.10.2:6443 

-------------------------------------------
"
sleep 1


#################################################
# initialize
#################################################
initialize(){
echo -e "
------ initializing phase-----
setting the kube config and frequency values
------------------------------
"

if [ "$2" = "" ]; then
echo "no KUBCONFIG is provided, using default '$HOME/.kube/conf'"
config="~/.kube/conf"
else 
    echo "using the following config: $config"
fi

if [ "$4" = "" ]; then
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"
echo "no cert path, using default '$SCRIPTPATH/certs', this may not be correct!!!"
certPath="$SCRIPTPATH/certs"
else 
    echo "using the following certPath: $certPath"
fi

if [ "$3" = "" ]; then
echo "no namespace is provided, using 'default'"
namespace="default"
else 
    echo "using the following config: $namespace"
fi

if [ "$5" = "" ]; then
frequency=10
echo "using the default frequency: $frequency seconds"
else 
    echo "using the following frequency: $frequency seconds"
fi

if [ "$6" = "" ]; then
k8s_api_ip_port="192.168.10.2:6443"
echo "WARNING: the api-server ip:port is not provided using a default that may not be accurate: $k8s_api_ip_port"
else 
    echo "using the following api-server ip:port: $k8s_api_ip_port"
fi

if [[ "$1" = "" ]]
then
  deployment_name=XQueueJob
  echo "using the default deployment name: $deployment_name"
else 
  echo "using the following deployment name: $deployment_name" 
fi
export KUBECONFIG=$config

deployment_exists=$(kubectl get deployment -n $namespace $deployment_name)
if [[ "$deployment_exists" != "" ]]
then
  kubectl label --overwrite deployment $deployment_name -n $namespace available_cpu="0"
  kubectl label --overwrite deployment $deployment_name -n $namespace available_memory="0"

  #debug
  deployment_label_cpu=$(kubectl get deployment -n $namespace $deployment_name -o jsonpath="{.metadata.labels.available_cpu}")
  deployment_label_memory=$(kubectl get deployment -n $namespace $deployment_name -o jsonpath="{.metadata.labels.available_memory}")
  echo "Cluster $deployment_name has $deployment_label_cpu CPU and $deployment_label_memory MEMORY available."
else
  echo "WARNING: The deployment named $deployment_name must be running."
fi


echo -e "
------ initializing completed-----
"

}

#################################################
# get available resources
#################################################
set_available_resources(){

echo -e "
------ get available resources -----
Calculate available resources
"
available_cpu="0"
available_memory="0"

# Stanzas
local allocatable_flag=false
local allocated_flag=false

# Allocatable stanza resources
local allocatable_cpu_raw=""
local allocatable_memory_raw=""
local allocatable_cpu_flag=false
local allocatable_memory_flag=false

# Allocatable stanza resources
local allocated_cpu_raw=""
local allocated_memory_raw=""
local allocated_cpu_flag=false
local allocated_memory_flag=false

local IFS_orig="${IFS}"
IFS=' :'

nodes=$(kubectl get nodes --output=custom-columns=NAME:.metadata.name --no-headers=true | tr '\n' ' ')

for node in $nodes
do
  # Allocatable stanza resources
  allocatable_cpu_raw=""
  allocatable_memory_raw=""
  allocatable_cpu_flag=false
  allocatable_memory_flag=false

  # Allocatable stanza resources
  allocated_cpu_raw=""
  allocated_memory_raw=""
  allocated_cpu_flag=false
  allocated_memory_flag=false

  describe_out_raw=$(kubectl describe node $node | grep -A 5 "Allocated\|Allocatable:")
  echo "${describe_out_raw}"
  describe_out=$(echo ${describe_out_raw} | tr '\n' ' ')
  for wd in $describe_out
  do
    #echo "word=${wd}."

    # Get allocated_memory_raw (used)
    if [[ "$allocated_memory_flag" = "true" ]]
    then
      #echo "allocated memory flag true if found."
      allocated_memory_raw="${wd}"
      allocated_memory_flag=false
      continue
    fi

    # Get allocated_cpu_raw (used)
    if [[ "$allocated_cpu_flag" = "true" ]]
    then
      #echo "allocated cpu flag true if found."
      allocated_cpu_raw="${wd}"
      allocated_cpu_flag=false
      continue
    fi

    # Get allocatable_memory_raw (capacity)
    if [[ "$allocatable_memory_flag" = "true" ]]
    then
      echo "allocatable memory flag true if found."
      allocatable_memory_raw="${wd}"
      allocatable_memory_flag=false
      continue
    fi

    # Get allocatable_cpu_raw (capacity)
    if [[ "$allocatable_cpu_flag" = "true" ]]
    then
      #echo "allocatable cpu flag true if found."
      allocatable_cpu_raw="${wd}"
      allocatable_cpu_flag=false
      continue
    fi

    # Find usage stanza
    if [[ "$allocated_flag" = "false" ]]
    then
      if [[ "${wd}" = "Allocated" ]]
      then
        allocatable_flag=false
        allocated_flag=true
        continue
      fi
    fi
  
    # Find capacity stanza
    if [[ "$allocatable_flag" = "false" ]]
    then
      if [[ "$wd" = "Allocatable" ]]
      then
        allocatable_flag=true
        allocated_flag=false
        continue
      fi
    fi
  
    # Found resource flag.  Determine if flag is for allocated or allocatable. 
    if [[ "$wd" = "memory" ]] 
    then
      if [[ "$allocated_flag" = "true" ]] && [[ "$allocated_memory_raw" = "" ]]
      then
        echo "setting allocated memory flag true."
        allocated_memory_flag=true
      elif [[ "$allocatable_flag" = "true" ]] && [[ "$allocatable_memory_raw" = "" ]]
      then
        #echo "setting allocatable memory flag true."
        allocatable_memory_flag=true
      fi
      continue
    fi
  
    # Found resource flag.  Determine if flag is for allocated or allocatable. 
    if [[ "$wd" = "cpu" ]] 
    then
      if [[ "$allocated_flag" = "true" ]] && [[ "$allocated_cpu_raw" = "" ]]
      then
        #echo "setting allocated cpu flag true."
        allocated_cpu_flag=true 
      elif [[ "$allocatable_flag" = "true" ]] && [[ "$allocatable_cpu_raw" = "" ]]
      then
        #echo "setting allocatable cpu flag true."
        allocatable_cpu_flag=true 
      fi
      continue
    fi
  done 
  # Remove units from values and normalize
  allocatable_cpu=$(($allocatable_cpu_raw * 1024))
  allocatable_memory=${allocatable_memory_raw%"Ki"}  
  
  m_units=$(echo $allocated_cpu_raw | grep m)
  if [[ "$m_units" != "" ]]
  then
    allocated_cpu=${allocated_cpu_raw%"m"}
  else
    allocated_cpu=$(($allocated_cpu_raw * 1024))
  fi

  meg_units=$(echo $allocated_memory_raw | grep Mi)
  if [[ "$meg_units" != "" ]]
  then
    allocated_memory=${allocated_memory_raw%"Mi"}
    allocated_memory=$(($allocated_memory * 1024))
  else
    gig_units=$(echo $allocated_memory_raw | grep Gi)
    if [[ "$gig_units" != "" ]]
    then
      allocated_memory=${allocated_memory_raw%"Gi"}
      allocated_memory=$(($allocated_memory * 1024000))
    else
      allocated_memory=$(($allocated_memory_raw * 1))
    fi
  fi

  # Calculate free space per node
  node_available_cpu=$(($allocatable_cpu - $allocated_cpu))
  node_available_memory=$(($allocatable_memory - $allocated_memory))

  # debug
  echo "allocatable cpu for node ${node} = ${allocatable_cpu}."
  echo "allocatable memory for node ${node} = ${allocatable_memory}."
  echo "allocated cpu for node ${node} = ${allocated_cpu}."
  echo "allocated memory for node ${node} = ${allocated_memory}."

  # Sum free space per cluster
  available_cpu=$(($available_cpu + $node_available_cpu))
  available_memory=$(($available_memory + $node_available_memory))
    
  echo "suming available cpu for cluster  = ${available_cpu}."
  echo "suming available  memory for cluster = ${available_memory}."
done

# Calculate free space and update cluster labels
kubectl label --overwrite deployment $deployment_name -n $namespace available_cpu=${available_cpu}
kubectl label --overwrite deployment $deployment_name -n $namespace available_memory=${available_memory}

# debug
deployment_label_cpu=$(kubectl get deployment -n $namespace $deployment_name -o jsonpath="{.metadata.labels.available_cpu}")
deployment_label_memory=$(kubectl get deployment -n $namespace $deployment_name -o jsonpath="{.metadata.labels.available_memory}")
echo "Cluster $deployment_name has $deployment_label_cpu CPU and $deployment_label_memory MEMORY available."

IFS="${IFS_orig}"
echo -e "
------ get available resources completed -----
"
}

#################################################
# ctrl_c:
#################################################
function ctrl_c() {
    echo -e "


 ____________________________________________
| stopping the cluster calculator, good bye! |
 --------------------------------------------
        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                
                ||     ||


"
    exit 1
}
#################################################
# Main
#################################################
trap ctrl_c INT

initialize $1 $2 $3 $4 $5

for (( ; ; ))
do
  set_available_resources
  sleep 10
done
