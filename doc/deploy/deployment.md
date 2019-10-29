# Deploying Multi-Cluster-App-Wrapper Controller
Follow the instructions below to deploy the Multi-Cluster-App-Wrapper controller in an existing Kubernetes cluster:

## Pre-Reqs
### - Cluster running Kubernetes v1.10 or higher.
```
kubectl version
```
### - Access to the `kube-system` namespace.
```
kubectl get pods -n kube-system
```
### - Install the Helm Package Manager
Install the Helm Client on your local machine and the Helm Cerver on your kubernetes cluster.  Helm installation documentation is [here]
(https://docs.helm.sh/using_helm/#installing-helm).  After you install Helm you can list the Help packages installed with the following command:
```
helm list
```

### Determine if the cluster has enough resources for installing the Helm chart for the Multi-Cluster-App-Dispatcher.

The default memory resource demand for the multi-cluster-app-dispatcher controller is `2G`.  If your cluster is a small installation such as MiniKube you will want to adjust the Helm installation resource requests accordingly.  


To list available compute nodes on your cluster enter the following command:
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
$ kubectl describe node <node_name>
```
For example:
```
$ kubectl describe node minikube
...
Name:               minikube
Roles:              master
Labels:             beta.kubernetes.io/arch=amd64
                    beta.kubernetes.io/os=linux
...
Capacity:
 cpu:                2
 ephemeral-storage:  16888216Ki
 hugepages-2Mi:      0
 memory:             2038624Ki
 pods:               110
Allocatable:
 cpu:                2
 ephemeral-storage:  15564179840
 hugepages-2Mi:      0
 memory:             1936224Ki
 pods:               110
...
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource  Requests      Limits
  --------  --------      ------
  cpu       1915m (95%)   1 (50%)
  memory    1254Mi (66%)  1364Mi (72%)
Events:     <none>

```
In the example above, there is only one node (`minikube`) in the cluster with the majority of the cluster memory used (`1,254Mi` used out of `1,936Mi` allocatable capacity) leaving less than `700Mi` available capacity for new pod deployments in the cluster.  Since the default memory demand for the Enhanced QueueuJob Controller pod is `2G` the cluster has insufficient memory to deploy the controller.  Instruction notes provided below show how to override the defaults according to the available capacity in your cluster.

## Installation Instructions
### 1. Download the github project.
Download this github project to your local machine.  
```
git clone -b queuejob-dispatcher --single-branch git@github.ibm.com:ARMS/extended-queuejob.git
```
### 2. Navigate to the Helm deployment directory.
```
cd extended-queuejob/contrib/DLaaS/deployment
```

### 3. Run the installation using Helm.
Install the Multi-Cluster-App-Dispatcher Controller using the commands below.  The `--wait` parameter in the Helm command below is  used to ensure all pods of the helm chart are running and will not return unless the default timeout expires (*typically 300 seconds*) or all the pods are in `Running` state.


Before submitting the command below you should ensure you have enough resources in your cluster to deploy the helm chart (*see Pre-Reqs section above*).  If you do not have enough compute resources in your cluster you can adjust the resource request via the command line.  See an example in the `Note` below.  

All Helm parameters and described in the table below.
#### 3.a  Start the Multi-Cluster-App-Dispatcher Controller on All Target Deployment Clusters (*Agent Mode*).
__Agent Mode__: Install and set up the Multi-Cluster-App-Dispatcher Controler (XQJ) in *Agent Mode* for each clusters that will orchestrate the resources defined within an XQJ using Helm.  *Agent Mode* is the default mode when deploying the XQJ controller.
```
helm install kube-arbitrator --namespace kube-system --wait --set image.repository=<image repository and name> --set image.tag=<image tag> --set imagePullSecret.name=<Name of image pull kubernetes secret> --set imagePullSecret.password=<REPLACE_WITH_REGISTRY_TOKEN_GENERATED_IN_PREREQs_STAGE1_REGISTRY.d)>  --set localConfigName=<Local Kubernetes Config File for Current Cluster>  --set volumes.hostPath=<Host_Path_location_of_local_Kubernetes_config_file>
```

For example (*Assuming the default for `image.repository`, `image.tag`*):
```
helm install kube-arbitrator --namespace kube-system
```
or
```
helm install kube-arbitrator --namespace kube-system --wait --set imagePullSecret.name=extended-queuejob-controller-registry-secret --set imagePullSecret.password=eyJhbGc...y8gJNcpnipUu0 --set image.pullPolicy=Always --set localConfigName=config_110 --set volumes.hostPath=/etc/kubernetes
```
NOTE: You can adjust the cpu and memory demands of the deployment with command line overrides.  For example:

```
helm install kube-arbitrator --namespace kube-system --wait -set resources.requests.cpu=1000m --set resources.requests.memory=1024Mi --set resources.limits.cpu=1000m --set resources.limits.memory=1024Mi --set image.repository=k8s-spark-mcm-dispatcher-master-1:8443/xqueuejob-controller --set image.tag=v1.11 --set image.pullPolicy=Always
```
#### 3.b  Start the Multi-Cluster-App-Dispatcher Controller on the Controller Cluster (*Dispatcher Mode*).
_Dispatcher Mode__: Install and set up the Multi-Cluster-App-Dispatcher Controler (XQJ) in *Dispatcher Mode* for the control cluster that will dispatch the XQJ to an *Agent* cluster using Helm.


__Dispatcher Mode__: Installing the Multi-Cluster-App-Dispatcher Controler in *Dispatcher Mode*.
```
helm install kube-arbitrator --namespace kube-system --wait --set image.repository=<image repository and name> --set image.tag=<image tag> --set configMap.name=<Config> --set configMap.dispatcherMode='"true"' --set configMap.agentConfigs=agent101config:uncordon --set volumes.hostPath=<Host_Path_location_of_all_agent_Kubernetes_config_files>
```

For example:
```
helm install kube-arbitrator --namespace kube-system --wait --set image.repository=tonghoon --set image.tag=both --set configMap.name=xqj-deployer --set configMap.dispatcherMode='"true"' --set configMap.agentConfigs=agent101config:uncordon --set volumes.hostPath=/etc/kubernetes
```
### Chart configuration

The following table lists the configurable parameters of the helm chart and their default values.

| Parameter               | Description                          | Default       | Sample values                                    |
| ----------------------- | ------------------------------------ | ------------- | ------------------------------------------------ |
| `configMap.agentConfigs`    | *For Every Agent Cluster separated by commas(,):* Name of *agent* config file _:_  Set the dispatching mode for the _*Agent Cluster*_.  Note:For the dispatching mode `uncordon`, indicating XQJ controller is allowed to dispatched jobs to the _*Agent Cluster*_, is only supported.  | &lt;_No default for agent config file_&gt;:`uncordon` | `agent101config:uncordon,agent110config:uncordon`      |
| `configMap.dispatcherMode`    | Whether the XQJ Controller should be launched in Dispatcher mode or not  | `false`  | `true`      |
| `configMap.name`    | Name of the Kubernetes *ConfigMap* resource to configure the Enhance QueueJob Controller   |   | `xqj-deployer`      |
| `deploymentName`      | Name of XQJ Controller Deployment Object | `xqueuejob-controller` | `my-xqj-controller` |
| `image.pullPolicy`     | Policy that dictates when the specified image is pulled    | `Always`  | `Never`      |
| `imagePullSecret.name`            | Kubernetes secret name to store password for image registry          |  | `extended-queuejob-controller-registry-secret`      |
| `imagePullSecret.password`            | Image registry pull secret password           |  | `eyJhbGc...y8gJNcpnipUu0`      |
| `imagePullSecret.username`            | Image registry pull user name           | `iamapikey` | `token`      |
| `image.repository`     | Name of repository containing XQueueJob Controller image    | `registry.stage1.ng.bluemix.net/ibm/kube-arbitrator`  | `my-repository`      |
| `image.tag`     | Tag of desired image within repository    | `latest`  | `my-image`      |
| `namespace`      | Namespace in which XQJ Controller Deployment is created | `kube-system` | `my-namespace` |
| `nodeSelector.hostname`    | Host Name field for XQJ Controller Pod Node Selector   |   | `example-host`      |
| `replicaCount`      | Number of replicas of XQJ Controller Deployment | 1 | 2 |
| `resources.limits.cpu`     | CPU Limit for XQJ Controller Deployment    | `2000m`  | `1000m`      |
| `resources.limits.memory`     | Memory Limit for XQJ Controller Deployment    | `2048Mi`  | `1024Mi`      |
| `resources.requests.cpu`     | CPU Request for XQJ Controller Deployment (must be less than CPU Limit)    | `2000m`  | `1000m`      |
| `resources.requests.memory`     | Memory Request for XQJ Controller Deployment (must be less than Memory Limit)   | `2048Mi`  | `1024Mi`      |
| `serviceAccount`    | Name of service account of XQJ Controller   | `xqueuejob-controller`  | `my-service-account`      |
| `volumes.hostPath`    | Full path on the host location where the `localConfigName` file is stored  |   | `/etc/kubernetes`      |


### 5. Verify the installation.
List the Helm installation.  The `STATUS` should be `DEPLOYED`.  

NOTE: The `--wait` parameter in the helm installation command from *step #3* above ensures all resources are deployed and running if the `STATUS` indicates `DEPLOYED`.  Installing the Helm Chart without the `--wait` parameter does not ensure all resources are successfully running but may still show a `Status` of `Deployed`.  

The `STATUS` value of `FAILED` indicates all resources were not created and running before the timeout occurred.  Usually this indicates a pod creation failure is due to insufficient resources to create the Multi-Cluster-App-Dispatcher Controller pod.  Example instructions on how to adjust the resources requested for the Helm chart are described in the `NOTE` comment of *step #4* above.
```
$ helm list
NAME                	REVISION	UPDATED                 	STATUS  	CHART                	NAMESPACE  
opinionated-antelope	1       	Mon Jan 21 00:52:39 2019	DEPLOYED	kube-arbitrator-0.1.0	kube-system

```

Ensure the new resource but listing the Extended QueueJobs.
```bash
kubectl get xqueuejobs
```

Since no `xqueuejobs` have been deploy yet to your cluster you should receive a message indicating `No resources found.` for `xqueuejobs` but your cluster now has `xqueuejobs` enabled.  Use the [tutorial](../doc/usage/tutorial.md) to deploy an example `xqueuejob`.

### 6.  Remove the Multi-Cluster-App-Dispatcher Controller from your cluster.

List the deployed Helm charts and identify the name of the Multi-Cluster-App-Dispatcher Controller installation.
```bash
helm list
```
For Example
```
$ helm list
NAME                	REVISION	UPDATED                 	STATUS  	CHART                	NAMESPACE  
opinionated-antelope	1       	Mon Jan 21 00:52:39 2019	DEPLOYED	kube-arbitrator-0.1.0	kube-system

```
Delete the Helm deployment.
```
helm delete <deployment_name>
```
For example:
```bash
helm delete opinionated-antelope
```
