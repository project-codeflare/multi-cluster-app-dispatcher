# Multi-Cluster-App-Deployer Controller Build Instructions

This document will show how to build the `Multi-Cluster-App-Deployer` Kubernetes Controller that operates on an AppWrapper kubernetes custom resource definition. It is for [master](https://github.com/IBM/multi-cluster-app-dispatcher/tree/master) branch.

## 1. Pre-condition

### Docker Environment

To build `Multi-Cluster-App-Deployer`, a running Docker env. must be available. Here is a document on [Getting Started with Docker](https://www.docker.com/get-started). 

### Clone Multi-Cluster-App-Deployer Git Repo

Clone this repo in your local environement:

```
$ git clone git@github.com:IBM/multi-cluster-app-dispatcher.git
Cloning into 'multi-cluster-app-dispatcher'...
Checking connectivity... done.
Checking out files: 100% (####/####), done.
$
```

## 2. Building the Multi-Cluster-App-Deployer Controller 

### Build the Executables 

Run the build script `build.sh`:
```
$ cd multi-cluster-app-dispatcher/deployment/

$ ./build.sh 
...
+ cd ..
+ make generate-code
Compiling deepcopy-gen
Generating deepcopy
go build -o _output/bin/deepcopy-gen ./cmd/deepcopy-gen/
_output/bin/deepcopy-gen -i ./pkg/apis/controller/v1alpha1/ -O zz_generated.deepcopy 
+ make kar-controller
mkdir -p _output/bin
CGO_ENABLED=0 GOARCH=amd64 go build -o _output/bin/kar-controllers ./cmd/kar-controllers/
$
```

Ensure the executables: `deepcopy-gen`, `kar-controllers`  are created in the target output directory:
```
$ ls ../_output/bin/
deepcopy-gen	kar-controllers
$
```

### Build the Multi-Cluster-App-Dispatcher

Run the image build scfript `image.sh`:

```
$ ./image.sh
...
+ make images
Changed to executable directory
Build the docker image
cd ./_output/bin
docker build --no-cache --tag mcad-controller:v1.14 ...
Sending build context to Docker daemon  122.7MB
Step 1/7 : From ubuntu:18.04
 ---> ea4c82dcd15a
Step 2/7 : ADD mcad-controller /usr/local/bin
 ---> 674cefbce55a
...
 ---> 911c7c82b5ee
Step 7/7 : WORKDIR /usr/local/bin
 ---> Running in f2db4649e7a6
Removing intermediate container f2db4649e7a6
 ---> 1dbf126976cf
Successfully built 1dbf126976cf
Successfully tagged mcad-controller:v1.14
$
$ docker images mcad-controller
REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
mcad-controller     deleteme            1dbf126976cf        11 minutes ago      272MB
$
```
