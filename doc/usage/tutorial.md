# Tutorial of Multi-Cluster Application Dispatcher Controller

This doc will show how to use the `multi-cluster-app-dispatcher` (`MCAD`) controller to dispatch the kubernetes custom resource definition `AppWrapper`. These instructions are based on the [master](https://github.com/IBM/multi-cluster-app-dispatcher/tree/master) branch.

## 1. Pre-condition
Runnning the `MCAD` controller requires a Kubernetes cluster to be available. Here is a document on [Using kubeadm to Create a Cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/). Additionally, for introduction purposes and testing, deploying on a small Kubernetes cluster such as Minikube can be used. This document shows examples of deploying `AppWrappers` custom resources with the `MCAD` controller deployed on a [Running Kubernetes Locally via Minikube](https://kubernetes.io/docs/getting-started-guides/minikube/).

The `multi-cluster-app-dispatcher` runs as a Kubernetes custom resource controller and operates on an `AppWrapper` custom resource definition. The next step will show how to deploy the `multi-cluster-app-dispatcher` as a Kubernetes custom resource definition quickly.

## 2. Deploy the Multi-Cluster Application Dispatcher Controller in Kubernetes

#### Deploy `multi-cluster-app-dispatcher` via Helm

Refer to [deployment instructions](../deploy/deployment.md) on how to deploy the `multi-cluster-app-dispatcher` as a controller in Kubernetes.

## 3. Create an AppWrapper Job

After successfully deploying the __Multi-Cluster Application Dispatcher__ Controller, create an AppWrapper custom resource in a file named `aw-01.yaml` with the following content:

```
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: stateful-set-2-replicas
spec:
  schedulingSpec:
    minAvailable: 2
  resources:
    Items:
    - replicas: 1
      type: StatefulSet
      template:
        apiVersion: apps/v1 # for versions before 1.9.0 use apps/v1beta2
        kind: StatefulSet
        metadata:
          name: stateful-set-2-replicas
          labels:
            app: stateful-set-2-replicas
        spec:
          selector:
            matchLabels:
              app: stateful-set-2-replicas
          replicas: 2
          template:
            metadata:
              labels:
                app: stateful-set-2-replicas
                size: "2"
            spec:
              containers:
               - name: stateful-set-2-replicas
                 image: kicbase/echo-server:1.0
                 resources:
                   requests:
                     memory: "200Mi"
                   limits:
                     memory: "200Mi"
```

The yaml file above is used to create an `AppWrapper` named `stateful-set-2-replicas` which encompasses a Kubernetes StatefulSet resulting in the creation of 2 pods.  These 2 pods will be scheduled by kubernetes scheduler.

Create the `AppWrapper` Job

```
$ kubectl create -f aw-01.yaml
```

Check `AppWrapper` job creation with the command below.  You should see the `stateful-set-2-replicas` job.

```
$ kubectl get appwrappers
NAME                      AGE
stateful-set-2-replicas   6s
$
```
Check `AppWrapper` job status by describing the `AppWrapper`.  The `Status:` stanza will show the `State` of `Running` if it has successfully deployed.

```
$ kubectl describe appwrapper stateful-set-2-replicas
Name:         stateful-set-2-replicas
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  mcad.ibm.com/v1beta1
Kind:         AppWrapper
. . .
Status:
  Canrun:  true
  State:   Running
```
Check the pods status of the `AppWrapper` job.  There should be two `stateful-set-2-replicas` pods running.

```
$ kubectl get pods
NAME                        READY   STATUS    RESTARTS   AGE
stateful-set-2-replicas-0   1/1     Running   0          3m
stateful-set-2-replicas-1   1/1     Running   0          3m
$
```

This step showed a simple deployment of an `AppWrapper` job.  The next step will show how queuing works in the `Multi-Cluster Application Dispatcher` Controller.
### 4. Demonstrating Queuing of an AppWrapper Job

When there are not enough aggregate resources available in the cluster to deploy the Kubernetes objects wrapped by the `AppWrapper` job, the `MCAD` Controller will queue the entire job (no partial deployments will be created).  This can benefit some batch workloads that require all resources to be deployed in order to make progress.  As an example, some distributed AI Deep Learning jobs define job parameters requiring all learners to be deployed, process and then communicate in a synchronous manner.  Partial deployment of these workloads results in inefficient resource allocation when a subset of resources are created but remain unused until all resources of the job can be deployed.

To demonstrate this you will need to identify a compute resource to use for demonstration.  Below is a simple example which can be used to demonstrate queuing of jobs until enough aggregate resources are available to deploy all resources defined in a job.

#### 4.a. Determine a Compute Resource for Demonstration

Most clusters have *cpu* and *memory* compute resources available for scheduling.  Below is an example of how to display compute resources one of which will be used to demonstrate queuing.

List available compute nodes in the cluster:
```
$ kubectl get nodes
```
For example:
```
$ kubectl get nodes
     NAME       STATUS    ROLES     AGE       VERSION
     minikube   Ready     master    91d       v1.10.0
```

To find out the available resources in you cluster inspect each node from the command output above with the following command:
```
$ kubectl describe node <node_name>
```
For example:

```
$ kubectl describe node minikube
. . .
Name:               minikube
. . .
Capacity:
 cpu:                2
 memory:             1986524Ki
 . . .

```
In the example above, there is only one node (`minikube`) in the cluster with both *cpu* and *memory* compute resources. Select one of the compute resources in your cluster to use for demonstration.  *Memory* will be used in this tutorial to demonstrate queuing.


#### 4.b. Create A Simple AppWrapper
If you still have the job running from [*Step #3*](#3-create-an-appwrapper-job) above then you can skip section 4.c.  If you have deleted job or skipped [*Step #3*](#3-create-an-appwrapper-job) above repeat [*Step #3*](#3-create-an-appwrapper-job) and then move section 4.c.

#### 4.c. Determine All Cluster Available Compute Resources After Creating the Simple AppWrapper Job from Step 3
To find out the capacity and un-allocated resources in you cluster inspect each node in your cluster with the following command:
```
$ kubectl describe node <node_name>
```

For example:

```
$ kubectl describe node minikube
```

```
. . .
Name:               minikube
. . .

Capacity
 cpu:                2
 memory:             1986524Ki
 pods:               110
Allocatable:
 cpu:                2
 memory:             1884124Ki
 pods:               110
 . . .
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource  Requests      Limits
  --------  --------      ------
  cpu       1755m (87%)   1 (50%)
  memory    1614Mi (87%)  1764Mi (95%)
Events:     <none>
. . .

```

In the example above, there is only one node (`minikube`) in the cluster with the majority of the cluster memory, `1,614Mi`, requested out of `1,884Mi` allocatable capacity, leaving less than `300Mi` available capacity for new pods to be deployed in the cluster.

#### 4.d. Create an AppWrapper Job to be Queued
The next step is to create a second `AppWrapper` job that has resource demands that would fit within the cluster capacity if the first job created in [*Step #3*](#3-create-an-appwrapper-job) was not allocated.  Continuing with the example above using cluster memory as the limiting resource, create a second `AppWrapper` job with the resource demands just described.  For example, since the example cluster has only one node, create a second `AppWrapper` job with a memory demand of `300Mi` which is slightly larger than the current available capacity.

Create an `AppWrapper` job in a file named `aw-02.yaml` with the following content:

```
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: deployment-2-replicas
spec:
  schedulingSpec:
    minAvailable: 2
  resources:
    Items:
    - replicas: 1
      type: Deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: deployment-2-replicas
          labels:
            app: deployment-2-replicas
        spec:
          selector:
            matchLabels:
              app: deployment-2-replicas
          replicas: 2
          template:
            metadata:
              labels:
                app: deployment-2-replicas
            spec:
              containers:
               - name: deployment-2-replicas
                 image: kicbase/echo-server:1.0
                 resources:
                   limits:
                     memory: 150Mi
                   requests:
                     memory: 150Mi
```

The yaml file above is used to create an `AppWrapper` named `deployment-2-replicas` which encompasses a Kubernetes Deployment resulting in the creation of 2 pods.  These 2 pods will be scheduled by kubernetes scheduler.

Create the `AppWrapper` Job

```
$ kubectl create -f aw-02.yaml
```

Check `AppWrapper` job creation with the command below.  You should see the `deployment-2-replicas` job.

```
$ kubectl get appwrappers
NAME                      AGE
deployment-2-replicas     10s
stateful-set-2-replicas   2h
```
Check job status by describing the job.  The `Status:` stanza will show the `State` of `Pending` if it has successfully queued.

```
$ kubectl describe appwrapper deployment-2-replicas
Name:         deployment-2-replicas
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  mcad.ibm.com/v1beta1
Kind:         AppWrapper
. . .
Status:
  Canrun:  true
  State:   Pending
```
Check the pods status of the `AppWrapper` jobs.  There should be only 2 pods from the first `AppWrapper` job `stateful-set-2-replicas` pods running and no existing pods from the second `AppWrapper` job, `deployment-2-replicas`.  

```
$ kubectl get pods
NAME                        READY   STATUS    RESTARTS   AGE
stateful-set-2-replicas-0   1/1     Running   0          3m
stateful-set-2-replicas-1   1/1     Running   0          3m
```
### Note: Since the `MCAD` controller has queued the `AppWrapper` of the second job the wrapped `Deployment` and related `Pod` objects do not get created in the Kubernetes cluster.  This results in effective utilization of the related Deployment Controller as well as the Kubernetes Scheduler since those controllers only operate on the objects when there are enough resources to actually run the `AppWrapper` job.

#### 4.e. Release the Queued `AppWrapper` Job
When enough resources in the cluster are released to run the queued job, the `MCAD` controler will dispatch the next queued job.  Continuing with the above example removing the first job will free up enough resources such that the `MCAD` controller can dispatch the second `AppWrapper` job.

Delete the first `AppWrapper` job.

```
$ kubectl delete -f aw-01.yaml
appwrapper.mcad.ibm.com "stateful-set-2-replicas" deleted
```

Check the pods status of the `AppWrapper` jobs.  The new pods from the second `AppWrapper` job: `deployment-2-replicas` job should now be deployed and running.  

```
$ kubectl get pods
NAME                                     READY   STATUS    RESTARTS   AGE
deployment-2-replicas-7bfd585bd4-vdgsk   1/1     Running   0          12s
deployment-2-replicas-7bfd585bd4-xkrs9   1/1     Running   0          12s
```
