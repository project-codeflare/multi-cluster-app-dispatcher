# Tutorial of Multi-Cluster Application Dispatcher Controller

This doc will show how to use the __Multi-Cluster Application Dispatcher__ (`MCAD`) controller to dispatch the kubernetes custom resource definition `AppWrapper`. These instructions are based on the [master](https://github.com/project-codeflare/multi-cluster-app-dispatcher/tree/main) branch.

## 1. Pre-condition

Runnning the `MCAD` controller requires a Kubernetes cluster to be available. Here is a document on [Using kubeadm to Create a Cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/). Additionally, for introduction purposes and testing, deploying on a small Kubernetes cluster such as Minikube can be used. This document shows examples of deploying `AppWrappers` custom resources with the `MCAD` controller deployed on a [Running Kubernetes Locally via Minikube](https://kubernetes.io/docs/getting-started-guides/minikube/).

The __Multi-Cluster Application Dispatcher__ runs as a Kubernetes custom resource controller and operates on an `AppWrapper` custom resource definition. The next step will show how to deploy the __Multi-Cluster Application Dispatcher__ as a Kubernetes custom resource definition quickly.

## 2. Deploy the Multi-Cluster Application Dispatcher Controller in Kubernetes

### Deploy __Multi-Cluster Application Dispatcher__ via Helm

Refer to [deployment instructions](../deploy/deployment.md) on how to deploy the __Multi-Cluster Application Dispatcher__ as a controller in Kubernetes.

## 3. Create an AppWrapper Job

After successfully deploying the __Multi-Cluster Application Dispatcher__ Controller, create an AppWrapper custom resource in a file named `aw-01.yaml` with the following content:

```yaml
apiVersion: workload.codeflare.dev/v1beta1
kind: AppWrapper
metadata:
  name: 0001-aw-generic-deployment-1
spec:
  resources:
    GenericItems:
    - replicas: 1
      generictemplate:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: 0001-aw-generic-deployment-1
          labels:
            app: 0001-aw-generic-deployment-1
        spec:
          selector:
            matchLabels:
              app: 0001-aw-generic-deployment-1
          replicas: 2
          template:
            metadata:
              labels:
                app: 0001-aw-generic-deployment-1
            spec:
              containers:
               - name: 0001-aw-generic-deployment-1
                 image: kicbase/echo-server:1.0
                 ports:
                 - containerPort: 80
                 resources:
                   requests:
                     cpu: 100m
                     memory: 256Mi
                   limits:
                     cpu: 100m
                     memory: 256Mi
```

The yaml file above is used to create an `AppWrapper` named `0001-aw-generic-deployment-1` which encompasses a Kubernetes Job resulting in the creation of 2 pods.  The appwrappers's pod will be scheduled by kubernetes scheduler.

Create the `AppWrapper`

```bash
kubectl create -f aw-01.yaml
```

Check `AppWrapper` job creation with the command below.  You should see the `0001-aw-generic-deployment-1` app wrapper.

```bash
$ kubectl get appwrappers
NAME                           AGE
0001-aw-generic-deployment-1   59s
```

Check `AppWrapper` job status by describing the `AppWrapper`.  The `Status:` stanza will show the `State` of `Running` if it has successfully deployed.

```bash
$ kubectl describe appwrapper 0001-aw-generic-deployment-1
...
Status:
  Canrun:  true
...
  State:      Running
```

Check the pods status of the `AppWrapper` job.  There should be 2 `0001-aw-generic-deployment-1` pods running.

```bash
$ kubectl get pods
NAME                                            READY   STATUS    RESTARTS   AGE
0001-aw-generic-deployment-1-79988f7ddd-bmzwb   1/1     Running   0          91s
0001-aw-generic-deployment-1-79988f7ddd-xkrxs   1/1     Running   0          91s
```

This step showed a simple deployment of an `AppWrapper` job.  The next step will show how queuing works in the __Multi-Cluster Application Dispatcher__ Controller.

### 4. Demonstrating Queuing of an AppWrapper Job

When there are not enough aggregate resources available in the cluster to deploy the Kubernetes objects wrapped by the `AppWrapper` job, the `MCAD` Controller will queue the entire job (no partial deployments will be created).  This can benefit some batch workloads that require all resources to be deployed in order to make progress.  As an example, some distributed AI Deep Learning jobs define job parameters requiring all learners to be deployed, process and then communicate in a synchronous manner.  Partial deployment of these workloads results in inefficient resource allocation when a subset of resources are created but remain unused until all resources of the job can be deployed.

To demonstrate this you will need to identify a compute resource to use for demonstration.  Below is a simple example which can be used to demonstrate queuing of jobs until enough aggregate resources are available to deploy all resources defined in a job.

#### 4.a. Determine a Compute Resource for Demonstration

Most clusters have *cpu* and *memory* compute resources available for scheduling.  Below is an example of how to display compute resources one of which will be used to demonstrate queuing.

List available compute nodes in the cluster:

```bash
kubectl get nodes
```

For example:

```bash
$ kubectl get nodes
NAME       STATUS   ROLES           AGE     VERSION
minikube   Ready    control-plane   7m15s   v1.26.3
```

To find out the available resources in you cluster inspect each node from the command output above with the following command:

```bash
kubectl describe node <node_name>
```

For example:

```bash
$ kubectl describe node minikube
Name:               minikube
Roles:              control-plane
...
Capacity:
Capacity:
  cpu:                2
  ephemeral-storage:  102625208Ki
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  hugepages-32Mi:     0
  hugepages-64Ki:     0
  memory:             8129036Ki
  pods:               110
```

In the example above, there is one node (`minikube`) in the cluster with both *cpu* and *memory* compute resources. Select one of the compute resources in your cluster to use for demonstration.  *Memory* will be used in this tutorial to demonstrate queuing.

#### 4.b. Create A Simple AppWrapper

If you still have the job running from [*Step #3*](#3-create-an-appwrapper-job) above then you can skip section 4.c.  If you have deleted job or skipped [*Step #3*](#3-create-an-appwrapper-job) above repeat [*Step #3*](#3-create-an-appwrapper-job) and then move section 4.c.

#### 4.c. Determine All Cluster Available Compute Resources After Creating the Simple AppWrapper Job from Step 3

To find out the capacity and un-allocated resources in you cluster inspect each node in your cluster with the following command:

```bash
kubectl describe node <node_name>
```

For example:

```bash
$ kubectl describe node minikube
Name:               minikube
...
Capacity:
  cpu:                2
  ephemeral-storage:  102625208Ki
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  hugepages-32Mi:     0
  hugepages-64Ki:     0
  memory:             8129036Ki
  pods:               110
Allocatable:
  cpu:                2
  ephemeral-storage:  102625208Ki
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  hugepages-32Mi:     0
  hugepages-64Ki:     0
  memory:             8129036Ki
  pods:               110
...
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests      Limits
  --------           --------      ------
  cpu                1950m (97%)   1200m (60%)
  memory             1706Mi (21%)  1706Mi (21%)
...
```

In the example above, there is one node (`minikube`) in the cluster with the majority of the cluster cpu capacity, `1950m`, requested out of `2000m` allocatable capacity, leaving less than `150m` available capacity for new pods to be deployed in the cluster.

#### 4.d. Create an AppWrapper Job to be Queued

The next step is to create a second `AppWrapper` job that has resource demands that would fit within the cluster capacity if the first job created in [*Step #3*](#3-create-an-appwrapper-job) was not allocated.  Continuing with the example above using cpu as the limiting resource, create a second `AppWrapper` job with the resource demands just described.  For example, since the example cluster has only one node, create a second `AppWrapper` job with a cpu demand of `200m` which is slightly larger than the current available capacity.

Create an `AppWrapper` job in a file named `aw-02.yaml` with the following content:

```yaml
apiVersion: workload.codeflare.dev/v1beta1
kind: AppWrapper
metadata:
  name: 0002-aw-generic-deployment-2
spec:
  schedulingSpec:
    minAvailable: 2
  resources:
    GenericItems:
    - replicas: 1
      generictemplate:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: 0002-aw-generic-deployment-2
          labels:
            app: 0002-aw-generic-deployment-2
        spec:
          selector:
            matchLabels:
              app: 0002-aw-generic-deployment-2
          replicas: 2
          template:
            metadata:
              labels:
                app: 0002-aw-generic-deployment-2
            spec:
              containers:
               - name: 0002-aw-generic-deployment-2
                 image: kicbase/echo-server:1.0
                 resources:
                   requests:
                     cpu: 75m
                     memory: 256Mi
                   limits:
                     cpu: 75m
                     memory: 256Mi
```

The yaml file above is used to create an `AppWrapper` named `0002-aw-generic-deployment-2` which encompasses a Kubernetes Deployment resulting in the creation of 2 pods.  These 2 pods will be scheduled by kubernetes scheduler.

Create the `AppWrapper` Job

```bash
kubectl create -f aw-02.yaml
```

Check `AppWrapper` job creation with the command below.  You should see the `deployment-2-replicas` job.

```bash
$ kubectl get appwrappers
NAME                           AGE
0001-aw-generic-deployment-1   104s
0002-aw-generic-deployment-2   23s
```

Check job status by describing the job.  The `Status:` stanza will show the `State` of `Pending` if it has successfully queued.

```bash
$ kubectl describe appwrapper 0002-aw-generic-deployment-2
Status:
  Conditions:
    Last Transition Micro Time:  2023-07-21T11:18:27.082148Z
    Last Update Micro Time:      2023-07-21T11:18:27.082148Z
    Status:                      True
    Type:                        Init
    Last Transition Micro Time:  2023-07-21T11:18:27.082164Z
    Last Update Micro Time:      2023-07-21T11:18:27.082164Z
    Reason:                      AwaitingHeadOfLine
    Status:                      True
    Type:                        Queueing
    Last Transition Micro Time:  2023-07-21T11:18:27.086747Z
    Last Update Micro Time:      2023-07-21T11:18:27.086747Z
    Reason:                      FrontOfQueue.
    Status:                      True
    Type:                        HeadOfLine
    Last Transition Micro Time:  2023-07-21T11:18:27.093222Z
    Last Update Micro Time:      2023-07-21T11:18:27.093222Z
    Message:                     Insufficient resources to dispatch AppWrapper.
    Reason:                      AppWrapperNotRunnable.
    Status:                      True
    Type:                        Backoff
  Controllerfirsttimestamp:      2023-07-21T11:18:27.082147Z
  Filterignore:                  true
  Number Of Requeueings:         0
  Queuejobstate:                 Backoff
  Requeueing Time In Seconds:    0
  Sender:                        before [backoff] - Rejoining
  State:                         Pending
```

Check the pods status of the `AppWrapper` jobs.  There should be only 2 pods from the first `AppWrapper` named `0001-aw-generic-deployment-1` pods running and no existing pods from the second `AppWrapper` named, `0002-aw-generic-deployment-2`.  

```bash
$ kubectl get pods
NAME                                            READY   STATUS    RESTARTS   AGE
0001-aw-generic-deployment-1-85ff6954bd-cw9ql   1/1     Running   0          5m30s
0001-aw-generic-deployment-1-85ff6954bd-vjbmb   1/1     Running   0          5m30s
```

Note: Since the `MCAD` controller has queued the `AppWrapper` of the second job the wrapped `Deployment` and related `Pod` objects do not get created in the Kubernetes cluster.  This results in effective utilization of the related Deployment Controller as well as the Kubernetes Scheduler since those controllers only operate on the objects when there are enough resources to actually run the `AppWrapper`.

#### 4.e. Release the Queued `AppWrapper` Job

When enough resources in the cluster are released to run the queued job, the `MCAD` controler will dispatch the next queued job.  Continuing with the above example removing the first job will free up enough resources such that the `MCAD` controller can dispatch the second `AppWrapper` job.

Delete the first `AppWrapper` job.

```bash
$ kubectl delete -f aw-01.yaml
workload.codeflare.dev/appwrapper "0001-aw-generic-deployment-1" deleted
```

Check the pods status of the `AppWrapper` jobs.  The new pods from the second `AppWrapper` job: `0002-aw-generic-deployment-2` job should now be deployed and running.  

```bash
$ kubectl get pods
NAME                                            READY   STATUS    RESTARTS   AGE
0002-aw-generic-deployment-2-66669c6484-6gw7z   1/1     Running   0          12s
0002-aw-generic-deployment-2-66669c6484-v5zl5   1/1     Running   0          12s
```
