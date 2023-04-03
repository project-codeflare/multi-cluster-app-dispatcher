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

From the root directory of the repository:

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

Ensure the executables: `deepcopy-gen`, `mcad-controllers`, and optionally `mca-controller-local` are created in the target output directory:

```bash
multi-cluster-app-dispatcher $ ls _output/bin 
deepcopy-gen            mcad-controller         mcad-controller-local
```

### Build the Multi-Cluster-App-Dispatcher Image

From the root directory of the repository:

```bash
# With docker daemon running
multi-cluster-app-dispatcher % make images

#Using podman
make images-podman
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

When running e2e tests, is recommended you restrict the `docker` daemon [cpu and memory resources](https://docs.docker.com/config/containers/resource_constraints/). The recomended settings are:

* CPU: 4
* Memory: 4 GB

From the root directory of the repository:

```bash
# With docker daemon running
multi-cluster-app-dispatcher % make run-e2e:
```
