#!/bin/bash

# Set container runtime system
if [ -x "$(command -v docker)" ]; then
    DOCKER=docker
else
    DOCKER=podman
fi

# Verify paramenter
if [ "$#" -ne "2" ]
then
    echo "Illegal number of parameters, requires 2."
    echo " "
    exit 2
fi
GIT_UID=${1:-GIT_UID}
GIT_TOKEN=${2:-GIT_TOKEN}

set -x

project_root=$(cd ..; pwd)
git_path=github.com/IBM/multi-cluster-app-dispatcher

set +x
echo "$DOCKER run  --rm -v $project_root:/go/src/$git_path -d -w /go/src/$git_path/deployment golang:1.16.3-alpine3.13 ./build-inside-container-private.sh"

container_id=$($DOCKER run --privileged --rm -v "$project_root":/go/src/$git_path -d -w /go/src/$git_path/deployment golang:1.16.3-alpine3.13 ./build-inside-container-private.sh ${GIT_UID} ${GIT_TOKEN})

set -x
$DOCKER logs -f $container_id
