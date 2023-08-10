# Performance Tests for MCAD with GPU workload
There are primarily two tests designed for testing MCAD's dispatching of GPU requesting jobs using KWOK.
1. With default K8 scheduler
2. With a coscheduler, for gang scheduling

For both cases, following metrics are captured:
1. Scheduling latency
2. Job completion time
3. Dispatch time (only for MCAD)
4. Response time 
5. GPU utilization
6. Pending pods

## Timestamped events of interest
1. Scheduling Latency: The interval between pod creation time and pod scheduled time.
```
% kubectl get pod mcadkwokcosched-gpu-job-1-54x8k -o yaml
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2023-08-03T13:07:27Z"
  finalizers:
  - kwok.x-k8s.io/fake
  generateName: mcadkwokcosched-gpu-job-1-
.
.
.
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-08-03T13:07:28Z"
    status: "True"
    type: Initialized
  - lastProbeTime: null
    lastTransitionTime: "2023-08-03T13:09:50Z"
    reason: PodCompleted
    status: "False"
    type: Ready
  - lastProbeTime: null
    lastTransitionTime: "2023-08-03T13:09:50Z"
    reason: PodCompleted
    status: "False"
    type: ContainersReady
  - lastProbeTime: null
    lastTransitionTime: "2023-08-03T13:07:28Z"
    status: "True"
    type: PodScheduled
.
.
.
```
With respect to scheduling latency, the timestamps of interest for the above pod spec are `creationTimestamp: "2023-08-03T13:07:27Z"` and `lastTransitionTime: "2023-08-03T13:07:28Z"` corresponding to `PodScheduled` event. 

2. Job Completion Time: The interval between Job creation time and Job completion time.
```
% kubectl get job mcadkwokcosched-gpu-job-1 -o yaml
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    batch.kubernetes.io/job-tracking: ""
  creationTimestamp: "2023-08-03T13:07:27Z"
  generation: 1
  labels:
    appwrapper.mcad.ibm.com: mcadkwokcosched-gpu-job-1
    resourceName: mcadkwokcosched-gpu-job-1
  name: mcadkwokcosched-gpu-job-1
.
.
.
status:
  completionTime: "2023-08-03T13:09:52Z"
  conditions:
  - lastProbeTime: "2023-08-03T13:09:52Z"
    lastTransitionTime: "2023-08-03T13:09:52Z"
    status: "True"
    type: Complete
  ready: 0
  startTime: "2023-08-03T13:07:27Z"
  .
  .
  .
```
For example, for this job spec, the timestamps captured are `creationTimestamp: "2023-08-03T13:07:27Z"` and `completionTime: "2023-08-03T13:09:52Z"`. The job completion time is then just `completionTime - creationTimestamp`.

3. Dispatch Time: The interval between AppWrapper creation time and dispatch time
```
% % kubectl get appwrapper mcadkwokcosched-gpu-job-1 -o yaml
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  creationTimestamp: "2023-08-03T13:07:27Z"
  generation: 6
  name: mcadkwokcosched-gpu-job-1
  namespace: default
.
.
.
status:
  canrun: true
  conditions:
  - lastTransitionMicroTime: "2023-08-03T13:07:27.384846Z"
    lastUpdateMicroTime: "2023-08-03T13:07:27.384840Z"
    status: "True"
    type: Init
  - lastTransitionMicroTime: "2023-08-03T13:07:27.386430Z"
    lastUpdateMicroTime: "2023-08-03T13:07:27.386426Z"
    reason: AwaitingHeadOfLine
    status: "True"
    type: Queueing
  - lastTransitionMicroTime: "2023-08-03T13:07:27.424254Z"
    lastUpdateMicroTime: "2023-08-03T13:07:27.424253Z"
    reason: FrontOfQueue.
    status: "True"
    type: HeadOfLine
  - lastTransitionMicroTime: "2023-08-03T13:07:29.295602Z"
    lastUpdateMicroTime: "2023-08-03T13:07:29.295602Z"
    reason: AppWrapperRunnable
    status: "True"
    type: Dispatched
.
.
.
```
The dispatch time is captured by the time difference between `creationTimestamp: "2023-08-03T13:07:27Z"` event and `lastTransitionMicroTime: "2023-08-03T13:07:29.295602Z"` for `Dipatched` event.

4. Response Time: The duration of time a request spends time in a queueing system. 
For a system with only scheduler, a job's response time is the time interval between when the job was created and when the job completed. This is same as Job completion time captured as described above.
For an MCAD queueing system, the response time for an AppWrapper (AW) can be defined as the duration for which the AW is tracked by the MCAD system. Intuitively, this time interval encompasses the period starting when the AW was first detected by the controller (as indicated by the controller's first timestamp) until the moment the MCAD system removes the AW from its bookkeeping or tracking records.
Considering this definition, the timestamp that corresponds to when MCAD removes the AppWrapper from its bookkeeping records can be referred to as the "controller removal timestamp" or "controller completion timestamp." This timestamp marks the point at which MCAD finalizes its handling of the AW, indicating that the AW has completed its lifecycle and is no longer being actively tracked or managed by the MCAD system.
This interval should capture the entire lifecycle of the AW within the MCAD system.

```
status:
  Succeeded: 1
  conditions:
  - lastTransitionMicroTime: "2023-08-08T20:58:42.065706Z"
    lastUpdateMicroTime: "2023-08-08T20:58:42.065705Z"
    status: "True"
    type: Init
  - lastTransitionMicroTime: "2023-08-08T20:58:42.066866Z"
    lastUpdateMicroTime: "2023-08-08T20:58:42.066865Z"
    reason: AwaitingHeadOfLine
    status: "True"
    type: Queueing
  - lastTransitionMicroTime: "2023-08-08T20:58:42.082548Z"
    lastUpdateMicroTime: "2023-08-08T20:58:42.082547Z"
    reason: FrontOfQueue.
    status: "True"
    type: HeadOfLine
  - lastTransitionMicroTime: "2023-08-08T20:58:42.539719Z"
    lastUpdateMicroTime: "2023-08-08T20:58:42.539718Z"
    reason: AppWrapperRunnable
    status: "True"
    type: Dispatched
  - lastTransitionMicroTime: "2023-08-08T20:58:52.976072Z"
    lastUpdateMicroTime: "2023-08-08T20:58:52.976071Z"
    reason: PodsRunning
    status: "True"
    type: Running
  - lastTransitionMicroTime: "2023-08-08T20:59:53.748149Z"
    lastUpdateMicroTime: "2023-08-08T20:59:53.748149Z"
    reason: PodsCompleted
    status: "True"
    type: Completed
  controllerfirsttimestamp: "2023-08-08T20:58:42.064572Z"
```
For this example, the response time of this AW is the time passed between `controllerfirsttimestamp: "2023-08-08T20:58:42.064572Z"` and `lastTransitionMicroTime: "2023-08-08T20:59:53.748149Z"` corresponding to `Completed` type event. 


## Test Run
The `run_sim.py` script is designed to simulate job requests in a Kubernetes system. It generates job requests based on specified parameters, applies them to the Kubernetes cluster and monitors their progress, and saves relevant information to output files.

### Prerequisites
- Python 3.x
- `kubectl` command-line tool configured to interact with the target Kubernetes cluster.
- MCAD controller installed. Follow [MCAD Deployment](https://github.com/project-codeflare/multi-cluster-app-dispatcher/blob/main/test/perf-test/simulatingnodesandappwrappers.md#step-1-deploy-mcad-on-your-cluster) to deploy and run MCAD controller on your cluster.
- Python packages used given in [requirements.txt](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/tree/main/test/kwok-tests/gpu-tests-kwok/requirements.txt)
- Optional: Co-scheduler [installation](https://github.com/vishakha-ramani/multi-cluster-app-dispatcher/tree/main/test/kwok-tests#appendix-installing-coscheduler)

### Usage
```
python3 run_sim.py [arguments]
```

### Command Line Arguments:

- --mean-arrival: Mean arrival time for job requests in seconds (default: 50).
- --total-jobs: Total number of job requests to generate (default: 100).
- --job-size: Mean sleep time of the container in seconds (default: 60).
- --output-file: Output file to store job results (default: job_results.txt).
- --pending-pod: Output file to store the number of pending pods (default: pending_pods.txt).
- --num-pod: Number of pods per job (default: 1).
- --gpu-options: GPU options for job requests (default: [2, 4, 6, 8]).
- --probabilities: Probabilities for GPU requirements (default: [0.25, 0.25, 0.25, 0.25]).
- --mode: Mode for job requests ('mcad' or 'nomcad', default: 'mcad').
- --israndom: Use random.expovariate for job request arrival and job size (default: True).
- --usecosched: Use coscheduler with the number of pods specified by --num-pod argument (default: False).


### Functionality:

The script simulates job request arrivals based on either fixed intervals or exponential distribution.
Job requests are generated with specified GPU requirements and job sizes.
The generated job requests are created as Kubernetes YAML files and applied to the cluster.
The script monitors the progress of job requests and the number of pending pods.
The script outputs job results and pending pod information to specified output files.
The script supports two modes: 'mcad' (with AppWrapper) and 'nomcad' (without AppWrapper).
The option to use coscheduler for job requests is available with the --usecosched flag.

### Example Usage:
1. To run an experiment with fixed job arrival duration and job size (set `--israndom False`), without co-scheduler (set `--usecosched False`), and MCAD mode (set `--mode mcad`):
```
python3 run_sim.py --mean-arrival 36 --total-jobs 50 --job-size 132 --gpu-options 8 --probabilities 1 --mode mcad --output-file mcadcosched_job_results.txt --pending-pod mcadcosched_pending_pod.txt --israndom False --usecosched False
```
In this scenario, 50 job requests will be generated with fixed interval of 36 seconds, each requiring 8 GPUs and having a fixed job size of 132 seconds. The script will output the results of the simulation to nomcadcosched_job_results.txt and the number of pending pods to nomcadcosched_pending_pod.txt. `MCAD` mode is on, and coscheduling is disabled.

2. To run an experiment with Poisson job arrivals and job size as exponential random variable (set `--israndom True`), with co-scheduler (set `--usecosched True`), and no MCAD mode (set `--mode nomcad`):
```
python3 run_sim.py --mean-arrival 36 --total-jobs 50 --job-size 132 --ßgpu-options 8 --probabilities 1 --mode nomcad --output-file nomcadcosched_job_results.txt --pending-pod nomcadcosched_pending_pod.txt --israndom True --usecosched True
```


### Note:
To analyze the aforementioned metrics for an experiment run, refer to `analysis-scripts` subdirectory. 
This script assumes that the appropriate Kubernetes configuration is already set up.
The behavior and details of the script may vary based on Kubernetes cluster settings and configurations.

### Disclaimer:

This script is intended for educational and simulation purposes only. Use it responsibly and only on test or sandbox environments.







## Appendix: Installing Coscheduler
1. Install co-scheduler using [official guide](https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/manifests/install/charts/as-a-second-scheduler/README.md#installing-the-chart)

2. Grant the appropriate RBAC (Role-Based Access Control) permissions to the service account "mcad-controller" in the "kube-system" namespace by running `kubectl apply -f` on the following:
```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcad-controller-podgroups
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: mcad-controller
  namespace: kube-system
```