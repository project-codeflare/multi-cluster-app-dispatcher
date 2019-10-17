#!/bin/bash
set -x

project_root=$(cd ..; pwd)
git_path=github.com/IBM/multi-cluster-app-dispatcher

container_id=$(docker run  --rm -v "$project_root":/go/src/$git_path -d -w /go/src/$git_path/deployment golang:alpine ./build-inside-container.sh)
#container_id=$(docker run  --rm -v "$project_root":/go/src/$git_path -d -w /go/src/$git_path/deployment golang:alpine sleep 999999999)

docker logs -f $container_id
