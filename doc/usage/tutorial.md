# Tutorial of Extended QueueJob Controller

This doc will show how to run `xqueuejob-controller` as a kubernetes custom resource definition. It is for [master](https://github.com/kubernetes-sigs/kube-batch/tree/master) branch.

## 1. Pre-condition
To run `xqueuejob-controller`, a running Kubernetes cluster must be available. Here is a document on [Using kubeadm to Create a Cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/). Additionally, for common purposes and testing and deploying on local machine, one can use Minikube. This is a document on [Running Kubernetes Locally via Minikube](https://kubernetes.io/docs/getting-started-guides/minikube/).

The `xqueuejob-controller` runs as a Kubernetes custom resource definition. The next step will show how to deploy the `xqueuejob-controller` as a Kubernetes custom resource definition quickly. 

## 2. Config kube-batch for Kubernetes

#### Deploy `xqueuejob-controller` by Helm

Follow the instructions [here](../../deployment/deployment.md) to install the `xqueuejob-controller` as a custom resource definition using [Helm](../../deployment/deployment.md).

### 3. Create a Job

After successfully deploying the Enhanced QueueJob Controller, create a file named `qj-02.yaml` with the following content:

```yaml
apiVersion: arbitrator.incubator.k8s.io/v1alpha1
kind: XQueueJob
metadata:
  name: helloworld-2
spec:
  schedSpec:
    minAvailable: 2
  resources:
    Items:
    - replicas: 1
      type: StatefulSet
      template:
        apiVersion: apps/v1 # for versions before 1.9.0 use apps/v1beta2
        kind: StatefulSet
        metadata:
          name: helloworld-2
          labels:
            app: helloworld-2
        spec:
          selector:
            matchLabels:
              app: helloworld-2
          replicas: 2 
          template:
            metadata:
              labels:
                app: helloworld-2
                size: "2" 
            spec:
              containers:
               - name: helloworld-2
                 image: gcr.io/hello-minikube-zero-install/hello-node
                 resources:
                   requests:
                     memory: "200Mi"
```

The yaml file above is used to a QueueJob named `helloworld-2` which encompases a stateful set with create 2 pods.  These 2 pods will be scheduled by kubernetes scheduler.

Create the Job

```
# kubectl create -f qj-02.yaml
```

Check job creation witht he command below.  You should see the `helloworld-2` job.

```
# kubectl get xqueuejobs
NAME                  CREATED AT
helloworld-2          7m
```
Check job status by describing the job.  The `Status:` stanza will show the `State` of `Running` if it has successfully deployed.

```
$ kubectl describe xqueuejob helloworld-2
Name:         helloworld-2
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  arbitrator.incubator.k8s.io/v1alpha1
Kind:         XQueueJob
. . .
Status:
  Canrun:  true
  State:   Running
```
Check the pods status of the job.  There shoud be 2 `helloworld-2` pods running.

```
# kubectl get pods
NAME             READY     STATUS    RESTARTS   AGE
helloworld-2-0   1/1       Running   0          7m
helloworld-2-1   1/1       Running   0          7m

```

This step showed a simple deployment of an Enhanced QueueJob.  The next step will show how queuing works in the Enhance QueueJob Controller.
### 4. Demonstrating Queuing of a QueueJob

When there is not enough aggregate resources available to deploy the all the resources of the queuejob within the cluster, the QueueJob Controller will queue the entire job (no partial deployments will be created).  This can benefit some batch workloads that require all resources to be deployed in order to make progress.  As an example, some distributed AI Deep Learning Jobs define job parameters requiring all learners to be deployed, process and then communicate in a synchronous manner.  Partial deployment of these workloads results in inefficient resource allocation as a subset of resource are created but remain unused until all resources of the job can be deployed. 

To demonstrate this you will need to identify a compute resource to use for demonstration.  Below is a simple example which can be used to demonstrate queuing of jobs until enough aggregate resources are available to deploy all resources defined in a job.

#### Determine a Compute Resource for Demonstration.

Most clusters have *cpu* and *memory* compute resources available for scheduling.  Below is an example of how to display compute resources one of which will be used to demonstrate queuing.

List available compute nodes in the cluster:
```
kubectl get nodes
```
For example:
```
$ kubectl get nodes
     NAME       STATUS    ROLES     AGE       VERSION
     minikube   Ready     master    91d       v1.10.0
```

To find out the available resources in you cluster inspect each node from the command output above with the following command:
```
kubectl describe node <node_name>
```
For example:
```
$ kubectl describe node minikube
...
Name:               minikube
...
Capacity:
 cpu:                2
 memory:             2038624Ki
 ...
 ```
In the example above, there is only one node (`minikube`) in the cluster with both *cpu* and *memory* compute resources. Select one of the compute resources in your cluster to use for demonstration.  *Memory* will be used in this tutorial to demonstrate queuing. 

#### Determine the Allocatable and Current Usage of the Compute Resource for Demonstration.

To find out the capacity and un-allocated resources in you cluster inspect each node in your cluster with the following command:
```
kubectl describe node <node_name>
```
For example:
```bash
$ kubectl describe node minikube
...
Name:               minikube
...
Capacity:
 cpu:                2
 memory:             2038624Ki
Allocatable:
 cpu:                2
 memory:             1936224Ki
...
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource  Requests      Limits
  --------  --------      ------
  cpu       1915m (95%)   1 (50%)
  memory    1254Mi (66%)  1364Mi (72%)
Events:     <none>

```
In the example above, there is only one node (`minikube`) in the cluster with the majority of the cluster memory used (`1,254Mi` used out of `1,936Mi` allocatable capacity) leaving less than `700Mi` available capacity for new pod deployments in the cluster.

#### Create A Simple Queue Job.
If you still have the job running from *Step #3* above then you can skip to the next section.  If have deleted job or skipped *Step #3* above repeat *Step #3* and then move to the next section.

#### Determine What Allocatable and Current Usage of All the Compute Resources After Creating the Simple Queue Job.
To find out the capacity and un-allocated resources in you cluster inspect each node in your cluster with the following command:
```
kubectl describe node <node_name>
```
For example:
```bash
$ kubectl describe node minikube
...
Name:               minikube
...
Capacity:
 cpu:                2
 memory:             2038624Ki
Allocatable:
 cpu:                2
 memory:             1936224Ki
...
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource  Requests      Limits
  --------  --------      ------
  cpu       1915m (95%)   1 (50%)
  memory    1254Mi (66%)  1364Mi (72%)
Events:     <none>

```
In the example above, there is only one node (`minikube`) in the cluster with the majority of the cluster memory used (`1,254Mi` used out of `1,936Mi` allocatable capacity) leaving less than `700Mi` available capacity for new pod deployments in the cluster.

