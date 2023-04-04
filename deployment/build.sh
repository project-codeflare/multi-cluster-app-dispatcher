#!/bin/bash
set -x

project_root=$(cd ..; pwd)
git_path=github.com/project-codeflare/multi-cluster-app-dispatcher

container_id=$(docker run  --rm -v "$project_root":/go/src/$git_path -d -w /go/src/$git_path/deployment golang:1.19 ./build-inside-container.sh)

docker logs -f $container_id
