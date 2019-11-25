Multi-Cluster Application Dispatcher
==

The `multi-cluster-app-dispatcher` is a Kubernetes controller providing mechanisms for applications to manage batch jobs. 

`multi-cluster-app-dispatcher` is capable of (i) providing an abstraction for wrapping all resources of the job/application and treating them holistically, (ii) queuing job/application creation requests and applying different queuing policies, e.g., FIFO, Priority, (iii) dispatching the job to one of multiple clusters, where a queuing agent runs, using configurable dispatch policies, and (iv) auto-scaling pod sets, balancing job demands and cluster availability (future work).


## Build Information

Follow the build instructions [here](./doc/build/build.md) to build the Multi-Cluster-App-Dispatcher controller.

## Deployment Information

Refer to [deployment](./doc/deploy/deployment.md) on how to deploy the `multi-cluster-app-dispatcher` as a controller in Kubernetes.

## Tutorial

Refer to the [tutorial](./doc/usage/tutorial.md) on how to use `multi-cluster-app-dispatcher` to run batch job in Kubernetes

## Overall Architecture

![xqueuejob-controler](doc/images/xqueuejob-controller.png)
