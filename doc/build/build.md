# Multi-Cluster-App-Dispatcher Controller Build Instructions

This document will show how to build the `Multi-Cluster-App-Dispatcher` (`MCAD`) Kubernetes Controller that operates on an `AppWrapper` kubernetes custom resource definition. Instructions are for the [main](https://github.com/project-codeflare/multi-cluster-app-dispatcher/tree/main) branch.

## 1. Pre-condition

### Docker Environment

To build `Multi-Cluster-App-Dispatcher`, a running Docker environment must be available. Here is a document on [Getting Started with Docker](https://www.docker.com/get-started). Podman image builds are also supported.

### Clone Multi-Cluster-App-Dispatcher Git Repo

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

* `Go` (version 1.18)
* `kind` (version 0.18)
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
mkdir -p _output/bin
Compiling controller
CGO_ENABLED=0 go build -o _output/bin/mcad-controller ./cmd/kar-controllers/
```

Ensure the executable `mcad-controllers` are created in the target output directory:

```bash
multi-cluster-app-dispatcher $ ls _output/bin 
mcad-controller
```

If you want pass additional args to the `go build`, define add them to the `GO_BUILD_ARGS` environment variable. This feature is useful if you want to compile the executable with the race condition detector turned on. To turn on the the race detector in your executable, execute:

```bash
make mcad-controller GO_BUILD_ARGS=-race
mkdir -p _output/bin
Compiling controller with build arguments: '-race'
go build -race -o _output/bin/mcad-controller ./cmd/kar-controllers/
```

### Build the Multi Cluster App Dispatcher Image

If you want to run the end to end tests locally, you will need to have the docker daemon running on your workstation, and build the image using docker. Images can also be build using podman for deployment of the MCAD controller on remote clusters.

From the root directory of the repository:

```bash
# With docker daemon running
multi-cluster-app-dispatcher % make images
....
make images
"---"
"MAKE GLOBAL VARIABLES:"
"  "BIN_DIR="_output/bin"
"  "GIT_BRANCH="main"
"  "RELEASE_VER="v1.29.57"
"  "TAG="main-v1.29.57"
"  "GO_BUILD_ARGS=""
"---"
# Check for invalid tag name
t=main-v1.29.57 && [ ${#t} -le 128 ] || { echo "Target name $t has 128 or more chars"; false; }
List executable directory
repo id: 
branch: main
Build the docker image
docker build --quiet --no-cache --tag mcad-controller:main-v1.29.57 -f XXXXXX/multi-cluster-app-dispatcher/Dockerfile  XXXXX/multi-cluster-app-dispatcher

#Using podman
make images-podman
....
# output from a main branch, MacOS build, local file names replaced with XXXXXXXXXX
"---"
"MAKE GLOBAL VARIABLES:"
"  "BIN_DIR="_output/bin"
"  "GIT_BRANCH="main"
"  "RELEASE_VER="v1.29.57"
"  "TAG="main-v1.29.57"
"  "GO_BUILD_ARGS=""
"---"
# Check for invalid tag name
t=main-v1.29.57 && [ ${#t} -le 128 ] || { echo "Target name $t has 128 or more chars"; false; }
List executable directory
repo id: 
branch: main
Build the docker image
podman build --quiet --no-cache --tag mcad-controller:main-v1.29.57 -f XXXXX/multi-cluster-app-dispatcher/Dockerfile  XXXXX/multi-cluster-app-dispatcher
```

The `GO_BUILD_ARGS` use is also supported by the images builds with either `docker` and `podman`. To turn on the race condition detector in image's executable execute: `make images GO_BUILD_ARGS=-race`

### Push the Multi-Cluster-App-Dispatcher Image to an Image Repository

The following example assumes an available `<repository>/mcad-controller` on [Docker Hub](https://hub.docker.com) and using image version `v1.14`

```bash
docker login
docker push <repository>/mcad-controller:v1.14
```

The same can be done with [Quay](quay.io)

```bash
docker login quay.io
docker push <quay_repository>/mcad-controller:v1.14
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
