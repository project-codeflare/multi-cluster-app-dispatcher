#!/bin/bash

current_dir=$(pwd)
cd ..
project_root=$(pwd)
cd $current_dir
docker run  --rm -v "$project_root":/go/src/github.com/kubernetes-sigs/kube-batch -d build-controller-image
#docker run  -v "$project_root":/go/src/github.com/kubernetes-sigs/kube-batch -d build-controller-image
