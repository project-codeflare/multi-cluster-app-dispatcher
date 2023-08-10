# Simulating Kubernetes Workloads with KWOK Toolkit #

The KWOK toolkit enables the simulation of the lifecycle of fake nodes, pods, and other Kubernetes API resources. This guide will walk you through the process of setting up and using KWOK to simulate custom pod lifecycles on a Kubernetes cluster. The example provided will use a KIND (Kubernetes IN Docker) cluster, but you can adapt it to any Kubernetes cluster of your choice.


## Installation ##

KWOK can be installed on Linux and macOS systems using the Homebrew package manager. Open a terminal and run the following command to install KWOK using Homebrew:
```
brew install kwok
```


## Simulating Fake Nodes ##

To start simulating fake nodes within your Kubernetes cluster, follow these steps:

1. Ensure you have a valid cluster login.
2. Run the [nodes.sh](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/blob/main/test/kwok-tests/nodes.sh) script, specifying the number of fake nodes you want to create:
```
% ./nodes.sh 
Checking whether we have a valid cluster login or not...

Nice, looks like you're logged in


How many simulated KWOK nodes do you want?4
Nodes number is 4
 
The real number of nodes is 3
Submitting node 1
node/kwok-node-1 created
Submitting node 2
node/kwok-node-2 created
Submitting node 3
node/kwok-node-3 created
Submitting node 4
node/kwok-node-4 created
Waiting until all the simualted pods become ready:
node/kwok-node-1 condition met
node/kwok-node-2 condition met
node/kwok-node-3 condition met
node/kwok-node-4 condition met
 
Total amount of simulated nodes requested is: 4
Total number of created nodes is:  4
NAME          STATUS   ROLES   AGE   VERSION
kwok-node-1   Ready    agent   1s    fake
kwok-node-2   Ready    agent   1s    fake
kwok-node-3   Ready    agent   1s    fake
kwok-node-4   Ready    agent   1s    fake
 
FYI, to clean up the kwow nodes, issue this:
kubectl get nodes --selector type=kwok -o name | xargs kubectl delete
```
3. Wait for the simulated fake nodes to become ready.

4. In the above run, we created `4` fake nodes. You can check the fake nodes running in the cluster by running:
```
% kubectl get nodes
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   17d   v1.27.1
kwok-node-1          Ready    agent           23s   fake
kwok-node-2          Ready    agent           23s   fake
kwok-node-3          Ready    agent           23s   fake
kwok-node-4          Ready    agent           23s   fake
```

Note that these fake nodes are not yet managed by the KWOK controller. The next section focusses on how to start the KWOK controller with a custom configuration.

## Starting KWOK controller that simulates custom pod lifecycle

In order to understand and simulate the lifecycle of a `Job`, we first design a baseline experiment on a `kind` cluster using only default kubernetes scheduler without MCAD. The `Job` spec looks as follows:

```
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default
  name: baseline-cpu-job-short-0
spec:
  parallelism: 1
  completions: 1
  template:
    metadata:
      namespace: default
    spec:
      containers:
      - args:
          - sleep
          - 10s
        name: baseline-cpu-job-short-0
        image: nginx:1.24.0
        resources:
            limits:
              cpu: 5m
              memory: 20M
            requests:
              cpu: 5m
              memory: 20M
      restartPolicy: Never
```

Here are the timings from the actual job on the `kind` cluster:
```
% kubectl get pods --output-watch-events --watch
EVENT      NAME                             READY   STATUS    RESTARTS   AGE
ADDED      baseline-cpu-job-short-0-75crs   0/1     Pending   0          0s
MODIFIED   baseline-cpu-job-short-0-75crs   0/1     Pending   0          0s
MODIFIED   baseline-cpu-job-short-0-75crs   0/1     ContainerCreating   0          0s
MODIFIED   baseline-cpu-job-short-0-75crs   1/1     Running             0          10s
MODIFIED   baseline-cpu-job-short-0-75crs   0/1     Completed           0          20s
MODIFIED   baseline-cpu-job-short-0-75crs   0/1     Completed           0          21s
MODIFIED   baseline-cpu-job-short-0-75crs   0/1     Completed           0          22s
```

Now, we would like to simulate similar timed events for a fake job as well. KWOK's `Stage` configuration ([link](https://kwok.sigs.k8s.io/docs/user/stages-configuration/)) allows users to define and simulate different stages in the lifecycle of pods. By configuring the `delay`, `selector`, and `next` fields in a `Stage`, you can control when and how the stage is applied, providing a flexible and scalable way to simulate real-world scenarios in your Kubernetes cluster. 

The YAML file [kwok.yaml](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/blob/main/test/kwok-tests/kwok.yaml) in this repo provides a custom pod lifecycle `Stage` API that simulates the events with the desired timings as described above. To run KWOK following the custom pod lifecycle, mount the configuration to `~/.kwok/kwok.yaml`, and then run KWOK controller out of your Kubernetes cluster (in my case it is a kind cluster) as follows:
```
kwok \
  --kubeconfig=~/.kube/config \
  --manage-all-nodes=false \
  --manage-nodes-with-annotation-selector=kwok.x-k8s.io/node=fake \
  --manage-nodes-with-label-selector= \
  --disregard-status-with-annotation-selector=kwok.x-k8s.io/status=custom \
  --disregard-status-with-label-selector= \
  --cidr=10.0.0.1/24 \
  --node-ip=10.0.0.1 \
--config=kwok.yaml
```

*Note 1*: If you have KWOK already deployed and running inside your Kubernetes cluster, make sure to first scale it down to avoid two instances of KWOK controller running. The command
```
kubectl scale deployment kwok-controller --replicas=0 -n kube-system
```
should scale down the kwok controller in the K8 cluster. 

*Note 2:* The above command essentially means that the KWOK controller will only manage nodes that have that specified annotation.

Now that we have KWOK running outside the cluster with our custom `Stage` configuration, we can create a fake `Job` and deploy it on the `kind` cluster. The fake `Job` spec looks as follows:
```
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default
  name: nomcadkwok-cpu-job-short-0
spec:
  parallelism: 1
  completions: 1
  template:
    metadata:
      namespace: default
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: type
                    operator: In
                    values:
                      - kwok
      # A taints was added to an automatically created Node.
      # You can remove taints of Node or add this tolerations.
      tolerations:
        - key: "kwok.x-k8s.io/node"
          operator: "Exists"
          effect: "NoSchedule"
      containers:
      - args:
          - sleep
          - 10s
        name: nomcadkwok-cpu-job-short-0
        image: nginx:1.24.0
        resources:
            limits:
              cpu: 5m
              memory: 50M
            requests:
              cpu: 5m
              memory: 50M
      restartPolicy: Never
```

The deployment of above `Job` spec creates following timed events:
```
% kubectl get pods --output-watch-events --watch
EVENT      NAME                               READY   STATUS    RESTARTS   AGE
ADDED      nomcadkwok-cpu-job-short-0-pkkqt   0/1     Pending   0          0s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   0/1     Pending   0          0s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   0/1     Pending   0          1s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   0/1     ContainerCreating   0          1s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   1/1     Running             0          11s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   0/1     Completed           0          22s
MODIFIED   nomcadkwok-cpu-job-short-0-pkkqt   0/1     Completed           0          23s
```

We can see that the lifecycle of the simulated job and an actual job matches. As a sanity check, one can also look at the fake `Pod` and fake `Job` description matches with actual `Pod` and `Job` description, except for tolerations and node affinity fields. 

# KWOK Tests
Based on templates for fake nodes, fake pods, and pod lifecycle, the repo provides primarily two tests for resource manager Multi-Cluster App Dispatcher [MCAD](https://github.com/project-codeflare/multi-cluster-app-dispatcher/tree/main):
1. Stress Tests with small jobs [stress-tests-kwok](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/tree/main/test/kwok-tests/stress-tests-kwok)
- We first demonstrate a major difference between actual Kubernetes Cluster and simulated Kubernetes cluster with KWOK.
- We then generalize the findings to understand the dispatch rate of MCAD in presence of large number of small jobs.
2. MCAD performance test [gpu-tests-kwok](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/tree/main/test/kwok-tests/gpu-tests-kwok)
- Includes profiling MCAD with metrics such as scheduling latency, number of pending pods, job completion time, dispatch time of requests.
