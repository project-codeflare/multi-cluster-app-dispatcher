## Stress Test 1: KWOK vs KIND
**Kind Cluster:** 4 nodes, each with 5 CPUs and 8 GB of memory
*We add node affinity to Pod spec to allow pods to be scheduled only on the worker nodes.*
In the `stress-baseline-cpu-j-s` directory, an example run looks as follows:
```
stress-baseline-cpu-j-s % ./baseline-avg-timing.sh -j 10 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```

**KWOK Cluster:** A cluster of 4 fake nodes managed by the KWOK controller is set up using the `./nodes.sh` script.
An example run within the `stress-nomcadkwok-cpu-j-s` directory shows:
```
stress-nomcadkwok-cpu-j-s % ./nomcadkwok-avg-timing.sh -j 10 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```
Both scripts take three arguments: `-j` for the number of jobs to be deployed, `-a` for the number of pods in a job, `-g` for the number of gpus per pod. Since this is a baseline experiment for small jobs, we set `-a 1` and `-g 0`.  The scripts give the average time it took to complete `j` number of jobs since the first job was submitted and one can play with parameter `-j` to understand the rate of completion with respect to different load.

**Performance Summary:**

| Jobs | Pods | Total time to complete (s) | Cluster Info (KIND)|
|------|------|--------------------------|-------------------------------------------------|
| 1    | 1    | 23                       |         KIND                                    |
| 10   | 10   | 23                       | with 4 worker nodes, job spec has node affinity |
| 20   | 20   | 27                       |                                                 |
| 50   | 50   | 33                       |                                                 |
| 75   | 75   | 41                       |                                                 |
| 100  | 100  | 49                       |                                                 |


| Jobs | Pods | Total time to complete (s) | Cluster Info (KWOK) |
|------|------|--------------------------|-----------------------------------------------------------------|
| 1    | 1    | 23                       |                                                                 |
| 10   | 10   | 23                       | 4 fake nodes, job spec has node affinity and tolerations        |
| 20   | 20   | 25                       |                                                                 |
| 50   | 50   | 31                       |                                                                 |
| 75   | 75   | 36                       |                                                                 |
| 100  | 100  | 40                       |                                                                 |



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



## Stress Test 2: Dispatch performance of MCAD with large number of small jobs
The aim of this experiment is to analyze the potential overhead introduced by the Multi-Cluster Application Dispatcher (MCAD) when dealing with a substantial volume of small application wrapper (AW) jobs, each consisting of a single pod. The key metric assessed in this experiment is the time taken for all jobs to complete. 
We use KWOK cluster with 4 nodes to test MCAD dispatch.


**MCAD Controller Deployment:** For this test, one should have MCAD controller deployed on the cluster. Follow [MCAD Deployment](https://github.com/project-codeflare/multi-cluster-app-dispatcher/blob/main/test/perf-test/simulatingnodesandappwrappers.md#step-1-deploy-mcad-on-your-cluster) to deploy and run MCAD controller on your cluster.

To run the stress test with KWOK and MCAD, go to `stress-mcad-cpu-j-s` directory, and run:
```
stress-mcad-cpu-j-s % ./mcad-avg-timing.sh -j 1 -g 0 -a 1
.
.
.
Average time per iteration: 23.00 seconds
```


The observed longer job creation times in MCAD are due to the additional processing that an AW undergoes before dispatching as a Job. AWs experience multiple queueing stages prior to being dispatched. After dispatching, the job is handled by the Kubernetes Job Controller. The slower creation time does not affect the actual time taken for the job to complete.
**Note: The dispatch rate of MCAD controller for this experiment looks like 1 job per second**

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
| Jobs   | Pods | Total time to complete (s) | Cluster Info                                              |
|--------|------|----------------------------|-----------------------------------------------------------|
| 1      | 1    | 23                         |                KWOK with MCAD                             |
| 10     | 10   | 31                         | 4 fake nodes, job spec has node affinity and tolerations  |
| 20     | 20   | 44                         |                                                           |
| 50     | 50   | 82                         |                                                           |
| 75     | 75   | 118                        |                                                           |
| 100    | 100  | 135                        |                                                           |

