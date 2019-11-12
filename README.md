Customized Enhanced QueueJob Controller Contribution
==

This directory is used to manage contribution for an enhanced queuejob controller from vendors.

# Multi Cluster App Dispatcher

The multi cluster app wrapper `multi-cluster-app-dispatcher` is a controller to manage batch jobs in Kubernetes, providing mechanisms for applications which would like to run batch jobs leveraging Kubernetes. 

`multi-cluster-app-dispatcher` is capable of (i) providing an abstraction for wrapping all resources of the job/application and treating them holistically, (ii) queuing job/application creation requests and applying different queuing policies, e.g., FIFO, Priority, (iii) dispatching the job to one of multiple clusters, where a queuing agent runs, using configurable dispatch policies, and (iv) auto-scaling pod sets, balancing job demands and cluster availability (future work).

Refer to [deployment](deployment/deployment.md) on how to deploy `multi-cluster-app-dispatcher` as in Kubernetes.

Refer to [tutorial](doc/usage/tutorial.md) on how to use `kube-batch` to run batch job in Kubernetes

## Multi-Cluster-App-Dispatcher Build Information

Follow the build instructions [here](./doc/build/build.md) to build the Multi-Cluster-App-Dispatcher controller.

## Overall Architecture

![xqueuejob-controler](doc/images/xqueuejob-controller.png)


### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
