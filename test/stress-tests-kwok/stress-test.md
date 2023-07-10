## Dispatch performance of MCAD with large number of small jobs
The following experiment was performed to understand the overhead associated with MCAD, if any, when large number of small AW jobs, consisting ony of 1 pod, are inserted into a system. To do that, we use KWOK toolkit that enables us to simulate the lifecycle of fake nodes, pods and other Kubernetes API resources. 

On Linux/MacOS systems you can install kwok/kwokctl via brew:
```
brew install kwok
```

In order to understand and simulate the lifecycle of a short `Job`, we first design a baseline experiment on a `kind` cluster using only default kubernetes scheduler without MCAD. The `Job` spec looks as follows:

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
      - name: baseline-cpu-job-short-0
        image: nginx:1.24.0
        command: ["sleep", "10"]
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

The YAML file `kwok.yaml` in this repo provides a custom pod lifecycle `Stage` API that simulates the events with the desired timings as described above. To run KWOK following the custom pod lifecycle, mount the configuration to `~/.kwok/kwok.yaml`, and then run KWOK out of your Kubernetes cluster (in my case it is a kind cluster) as follows:
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

*Note*: If you have KWOK already deployed and running inside your Kubernetes cluster, make sure to first scale it down to avoid two instances of KWOK controller running. The command
```
kubectl scale deployment kwok-controller --replicas=0 -n kube-system
```
should scale down the kwok controller in the K8 cluster. 

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
      - name: nomcadkwok-cpu-job-short-0
        image: nginx:1.24.0
        command: ["sleep", "10"]
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

Now that we have KWOK setup properly, we would like to see the performance comparison between simulated Kubernetes environment created by KWOK and actual Kubernetes environment of the Kind cluster. 


### Stress Test 1: KWOK vs KIND
The performance metric of interest is the `Job` completion time as a function of workload. 
**Kind Cluster:** 5 nodes, each with 5 CPUs and 8 GB of memory
*We add node affinity to Pod spec to allow pods to be scheduled only on the worker nodes.*
In the `stress-baseline-cpu-j-s` directory, an example run looks as follows:
```
stress-baseline-cpu-j-s % ./baseline-avg-timing.sh -j 10 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```

**KWOK Cluster:** 4 fake nodes that can be deployed by running `./nodes.sh`
Check the number of desired nodes in the cluster by 
```
kubectl get nodes --selector type=kwok -o name | wc -l
```
In the `stress-nomcadkwok-cpu-j-s` directory, an example run looks as follows:
```
stress-nomcadkwok-cpu-j-s % ./nomcadkwok-avg-timing.sh -j 10 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```
Both scripts take three arguments: `-j` for the number of jobs to be deployed, `-a` for the number of pods in a job, `-g` for the number of gpus per pod. Since this is a baseline experiment for small jobs, I set `-a 1` and `-g 0`.  The scripts give the average time it took to complete `j` number of jobs since the first job was submitted and one can play with parameter `-j` to understand the rate of completion with respect to different load.

**Performance Summary:**

| Date   | Tester | Jobs | Pods | Total time to complete (s) | Cluster Info (KIND)                                 |
|--------|--------|------|------|--------------------------|-------------------------------------------------|
| 3-Jul  | VR     | 1    | 1    | 23                       |         KIND                                        |
|        |        | 10   | 10   | 23                       | with 4 worker nodes, job spec has node affinity |
|        |        | 20   | 20   | 27                       |                                                 |
|        |        | 50   | 50   | 33                       |                                                 |
|        |        | 75   | 75   | 41                       |                                                 |
|        |        | 100  | 100  | 49                       |                                                 |


| Date   | Tester | Jobs | Pods | Total time to complete (s) | Cluster Info (KWOK)                                                   |
|--------|--------|------|------|--------------------------|-----------------------------------------------------------------|
| 3-Jul  | VR     | 1    | 1    | 23                       |                                                                 |
|        |        | 10   | 10   | 23                       | KWOK with 4 fake nodes, job spec has node affinity and tolerations |
|        |        | 20   | 20   | 25                       |                                                                 |
|        |        | 50   | 50   | 31                       |                                                                 |
|        |        | 75   | 75   | 36                       |                                                                 |
|        |        | 100  | 100  | 40                       |                                                                 |



The difference in KIND and KWOK completion times can be majorly attributed to the container start time. One can see that as the number of job requests increases, the maximum container start time also increases. For example, when the workload consists of submitting `10` jobs to the Kind cluster, we see the following 
```
% kubectl get pods -o json | python3 -c 'import sys, json, datetime; pods = json.load(sys.stdin)["items"]; [print("POD NAME", "PODSTARTTIME", "CONTAINERSTARTTIME", "CONTAINERFINISHTIME", "TIMETOSTART")] + [print(pod["metadata"]["name"], pod["status"]["startTime"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["finishedAt"], (datetime.datetime.fromisoformat(pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"].replace("Z", "+00:00")) - datetime.datetime.fromisoformat(pod["status"]["startTime"].replace("Z", "+00:00")))) for pod in pods]' 

POD NAME PODSTARTTIME CONTAINERSTARTTIME CONTAINERFINISHTIME TIMETOSTART
baseline-cpu-job-short-1-k6gm6 2023-07-06T13:54:49Z 2023-07-06T13:54:57Z 2023-07-06T13:55:07Z 0:00:08
baseline-cpu-job-short-10-ljwds 2023-07-06T13:54:50Z 2023-07-06T13:55:00Z 2023-07-06T13:55:10Z 0:00:10
.
.
.
```

However, when `100` jobs are submitted, then:

```
% kubectl get pods -o json | python3 -c 'import sys, json, datetime; pods = json.load(sys.stdin)["items"]; [print("POD NAME", "PODSTARTTIME", "CONTAINERSTARTTIME", "CONTAINERFINISHTIME", "TIMETOSTART")] + [print(pod["metadata"]["name"], pod["status"]["startTime"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["finishedAt"], (datetime.datetime.fromisoformat(pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"].replace("Z", "+00:00")) - datetime.datetime.fromisoformat(pod["status"]["startTime"].replace("Z", "+00:00")))) for pod in pods]'

POD NAME PODSTARTTIME CONTAINERSTARTTIME CONTAINERFINISHTIME TIMETOSTART
.
.
.
baseline-cpu-job-short-31-xvlt4 2023-07-06T14:10:05Z 2023-07-06T14:10:25Z 2023-07-06T14:10:35Z 0:00:20
baseline-cpu-job-short-32-w69jt 2023-07-06T14:10:05Z 2023-07-06T14:10:30Z 2023-07-06T14:10:41Z 0:00:25
baseline-cpu-job-short-33-5kptg 2023-07-06T14:10:05Z 2023-07-06T14:10:31Z 2023-07-06T14:10:41Z 0:00:26
baseline-cpu-job-short-34-5pxth 2023-07-06T14:10:05Z 2023-07-06T14:10:30Z 2023-07-06T14:10:41Z 0:00:25
baseline-cpu-job-short-35-48gr9 2023-07-06T14:10:05Z 2023-07-06T14:10:30Z 2023-07-06T14:10:41Z 0:00:25
.
.
.
```

For KWOK on the contrary, the container start time is simulated with fixed delay, and moreover there is no Kubelet that becomes a bottleneck in KIND. Therefore, we see almost constant container start time in KWOK.
*With 10 jobs:*
```
% kubectl get pods -o json | python3 -c 'import sys, json, datetime; pods = json.load(sys.stdin)["items"]; [print("POD NAME", "PODSTARTTIME", "CONTAINERSTARTTIME", "CONTAINERFINISHTIME", "TIMETOSTART")] + [print(pod["metadata"]["name"], pod["status"]["startTime"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["finishedAt"], (datetime.datetime.fromisoformat(pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"].replace("Z", "+00:00")) - datetime.datetime.fromisoformat(pod["status"]["startTime"].replace("Z", "+00:00")))) for pod in pods]'

POD NAME PODSTARTTIME CONTAINERSTARTTIME CONTAINERFINISHTIME TIMETOSTART
nomcadkwok-cpu-job-short-1-knfcg 2023-07-06T14:33:55Z 2023-07-06T14:34:06Z 2023-07-06T14:34:16Z 0:00:11
nomcadkwok-cpu-job-short-10-52c6c 2023-07-06T14:33:55Z 2023-07-06T14:34:06Z 2023-07-06T14:34:17Z 0:00:11
.
.
.
```

*With 100 jobs:* 
```
% kubectl get pods -o json | python3 -c 'import sys, json, datetime; pods = json.load(sys.stdin)["items"]; [print("POD NAME", "PODSTARTTIME", "CONTAINERSTARTTIME", "CONTAINERFINISHTIME", "TIMETOSTART")] + [print(pod["metadata"]["name"], pod["status"]["startTime"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"], pod["status"]["containerStatuses"][0]["state"]["terminated"]["finishedAt"], (datetime.datetime.fromisoformat(pod["status"]["containerStatuses"][0]["state"]["terminated"]["startedAt"].replace("Z", "+00:00")) - datetime.datetime.fromisoformat(pod["status"]["startTime"].replace("Z", "+00:00")))) for pod in pods]'

POD NAME PODSTARTTIME CONTAINERSTARTTIME CONTAINERFINISHTIME TIMETOSTART
nomcadkwok-cpu-job-short-1-jvbgk 2023-07-06T14:41:38Z 2023-07-06T14:41:49Z 2023-07-06T14:41:59Z 0:00:11
nomcadkwok-cpu-job-short-10-5bmlh 2023-07-06T14:41:39Z 2023-07-06T14:41:50Z 2023-07-06T14:42:00Z 0:00:11
nomcadkwok-cpu-job-short-100-w5wjv 2023-07-06T14:41:49Z 2023-07-06T14:42:00Z 2023-07-06T14:42:10Z 0:00:11
.
.
.
```
And the job creation rate is `10` jobs per second. So this means that for the stress load of 100 jobs, job 100 will be created after 10 seconds of job 1 and therefore the completion time accounts for this. The appropriate command to see this is. Command number 1.

### Stress Test 2: KWOK with and without MCAD: Single Pod

Next, we compare the completion time between No-MCAD and MCAD systems, we see that MCAD has longer completion times. In both cases, we have KWOK controller running outside the cluster using config file `kwok.yaml`
**KWOK Cluster:** 4 fake nodes that can be deployed by running `./nodes.sh`
Check the number of desired nodes in the cluster by 
```
kubectl get nodes --selector type=kwok -o name | wc -l
```

**MCAD Controller Deployment:** For this test, one should have MCAD controller deployed on the cluster. Follow [MCAD Deployment](https://github.com/project-codeflare/multi-cluster-app-dispatcher/blob/main/test/perf-test/simulatingnodesandappwrappers.md#step-1-deploy-mcad-on-your-cluster) to deploy and run MCAD controller on your cluster.

To run the stress test with KWOK and MCAD, go to `stress-mcad-cpu-j-s` directory, and run:
```
stress-mcad-cpu-j-s % ./mcad-avg-timing.sh -j 1 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```


We see that the job creation time in MCAD is longer. This is because MCAD is doing a bunch of things with the AW before dispatching as a Job. Particularly an AW has to go through multiple queueing stages before it is finally dispatched. After being dispatched, the job is handled by the Job Controller in K8, and from there on MCAD and No-MCAD systems should behave in the same way. 
*The dispatch rate of MCAD controller for this experiment looks like 1 job per second*

```
% kubectl get appwrapper -o custom-columns=AWNAME:.metadata.name,AWCREATED:.status.controllerfirsttimestamp
AWNAME                      AWCREATED
mcadkwok-cpu-job-short-1    2023-07-06T15:16:32.524203Z
.
.
.
mcadkwok-cpu-job-short-50   2023-07-06T15:16:44.335615Z
```

```
% kubectl get jobs -o custom-columns=JOBNAME:.metadata.name,JOBCREATED:.metadata.creationTimestamp,JOBCOMPLETED:.status.completionTime
JOBNAME                     JOBCREATED             JOBCOMPLETED
mcadkwok-cpu-job-short-1    2023-07-06T15:16:32Z   2023-07-06T15:16:55Z
.
.
.
mcadkwok-cpu-job-short-50   2023-07-06T15:17:15Z   2023-07-06T15:17:37Z
```

**Performance Summary of KWOK with MCAD**
| Date   | Tester | Jobs | Pods | Total time to complete (s) | Cluster Info                                              |
|--------|--------|------|------|--------------------------|-----------------------------------------------------------|
| 3-Jul  | VR     | 1    | 1    | 23                       |                                                           |
|        |        | 10   | 10   | 31                       | KWOK with MCAD 4 fake nodes, job spec has node affinity and tolerations |
|        |        | 20   | 20   | 44                       |                                                           |
|        |        | 50   | 50   | 82                       |                                                           |
|        |        | 75   | 75   | 118                      |                                                           |
|        |        | 100  | 100  | 135                      |                                                           |


### Stress Test 3: KWOK with and without MCAD: Single Pod



Some baseline results for this experiment can be found [here](https://ibm.box.com/s/glxzte7g4rtk1ew0i7gc17kxugyqn59j)