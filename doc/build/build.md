# Multi-Cluster-App-Dispatcher Controller Build Instructions

This document will show how to build the `Multi-Cluster-App-Deployer` (`MCAD`) Kubernetes Controller that operates on an `AppWrapper` kubernetes custom resource definition. Instructions are for the [master](https://github.com/IBM/multi-cluster-app-dispatcher/tree/master) branch.

## 1. Pre-condition

### Docker Environment

To build `Multi-Cluster-App-Deployer`, a running Docker environment must be available. Here is a document on [Getting Started with Docker](https://www.docker.com/get-started).

### Clone Multi-Cluster-App-Deployer Git Repo

Clone this repo in your local environment:

__Option 1__: Clone this github project to your local machine via HTTPS

```bash
$ git clone https://github.com/project-codeflare/multi-cluster-app-dispatcher.git
Cloning into 'multi-cluster-app-dispatcher'...
Checking connectivity... done.
Checking out files: 100% (####/####), done.
$
```

__Option 2__: Clone this github project to your local machine via SSH

```bash
$ git clone git@github.com:project-codeflare/multi-cluster-app-dispatcher.git
Cloning into 'multi-cluster-app-dispatcher'...
Checking connectivity... done.
Checking out files: 100% (####/####), done.
$
```

### Additional software needed

To build the controller and to run the end to end tests locally you will need to have the following software installed:

* `Go` (version 1.16) -- the controller will compile and run with later versions, but currently supported version is 1.16
* `kind` (version 0.11) -- later versions will work fine
* `kubectl`
* `helm` - version 3.0 or later
* `make`

On MacOS you will need to have `readlink` executable installed (`brew install coreutils`)

## 2. Building the Multi-Cluster-App-Deployer Controller

### Build the Executable

From the root directory of the repository, you may build only the executable, or you can build the image directly.

To to build the executable, execute:

```bash
#build for linux OS and for use inside docker image
multi-cluster-app-dispatcher $ make mcad-controller
...
Compiling deepcopy-gen...
Generating deepcopy...
go build -o _output/bin/deepcopy-gen ./cmd/deepcopy-gen/
_output/bin/deepcopy-gen -i ./pkg/apis/controller/v1beta1/ -O zz_generated.deepcopy 
Compiling controller
CGO_ENABLED=0 GOOS="linux" go build -o _output/bin/mcad-controller ./cmd/kar-controllers/

#build for local testing purposes, by default enable the race conditions detector
multi-cluster-app-dispatcher $ make mcad-controller-local
...
mkdir -p _output/bin
Compiling deepcopy-gen...
Generating deepcopy...
go build -o _output/bin/deepcopy-gen ./cmd/deepcopy-gen/
_output/bin/deepcopy-gen -i ./pkg/apis/controller/v1beta1/ -O zz_generated.deepcopy 
Compiling controller
go build -race -o _output/bin/mcad-controller-local ./cmd/kar-controllers/
```

Ensure the executables: `deepcopy-gen` and `mcad-controllers` are created in the target output directory:

```bash
multi-cluster-app-dispatcher $ ls _output/bin 
deepcopy-gen            mcad-controller
```

### Build the Multi-Cluster-App-Dispatcher Image

If you want to run the end to end tests locally, you will need to have the docker daemon running on your workstation, and build the image using docker. Images can also be build using podman for deployment of the MCAD controller on remote clusters.

From the root directory of the repository:

```bash
# With docker daemon running
multi-cluster-app-dispatcher % make images
....
# output from main branch, MacOS build, local file names replaced with XXXXXXXXXX
"---"
"MAKE GLOBAL VARIABLES:"
"  "BIN_DIR="_output/bin"
"  "GIT_BRANCH="main"
"  "RELEASE_VER="v1.29.55"
"  "TAG="main-v1.29.55"
"---"
# Check for invalid tag name
t=main-v1.29.55 && [ ${#t} -le 128 ] || { echo "Target name $t has 128 or more chars"; false; }
List executable directory
repo id: 
branch: main
Build the docker image
docker build --quiet --no-cache --tag mcad-controller:main-v1.29.55 -f XXXXXXXXXX/multi-cluster-app-dispatcher/Dockerfile  XXXXXXXXXX/multi-cluster-app-dispatcher
sha256:6871c150701280abc29baa14aa639791cefb9ba4b61177ab4faf5a43bdfcc4e4

#Using podman
make images-podman
....
# output from a main branch, MacOS build, local file names replaced with XXXXXXXXXX
"---"
"MAKE GLOBAL VARIABLES:"
"  "BIN_DIR="_output/bin"
"  "GIT_BRANCH="main"
"  "RELEASE_VER="v1.29.55"
"  "TAG="main-v1.29.55"
"---"
# Check for invalid tag name
t=main-v1.29.55 && [ ${#t} -le 128 ] || { echo "Target name $t has 128 or more chars"; false; }
List executable directory
repo id: 
branch: main
Build the docker image
ls -l XXXXXXXXXX/multi-cluster-app-dispatcher/_output/bin
total 130144
-rwxr-xr-x  1 laurentiu.bradin  staff   8238498 Apr  6 15:19 deepcopy-gen
-rwxr-xr-x  1 laurentiu.bradin  staff  58391090 Apr  6 15:19 mcad-controller
podman build --quiet --no-cache --tag mcad-controller:issue_315_small_changes-v1.29.55 -f XXXXXXXXXX/multi-cluster-app-dispatcher/Dockerfile  XXXXXXXXXX/multi-cluster-app-dispatcher
f784707e8982399ef7ef66e3d8a09b669e6deb17990d174400338813fb13c505
```

### Push the Multi-Cluster-App-Dispatcher Image to an Image Repository

The following example assumes an available `<repository>/mcad-controller` on [Docker Hub](https://hub.docker.com) and using image version `v1.14`

```bash
docker login
docker push <respository>/mcad-controller:v1.14
```

The same can be done with [Quay](quay.io)

```bash
docker login quay.io
docker push <quay_respository>/mcad-controller:v1.14
```

Refer to [deployment](../deploy/deployment.md) on how to deploy the `multi-cluster-app-dispatcher` as a controller in Kubernetes.

## 3. Running e2e tests locally

When running e2e tests, is recommended you restrict the `docker` daemon [cpu and memory resources](https://docs.docker.com/config/containers/resource_constraints/). The recommended settings are:

* CPU: 2
* Memory: 8 GB

From the root directory of the repository:

```bash
# With docker daemon running
multi-cluster-app-dispatcher % make run-e2e
```
