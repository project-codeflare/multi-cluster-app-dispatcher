There are primarily two tests designed for testing MCAD's dispatching of GPU requesting jobs using KWOK.
1. With default K8 scheduler
2. With a coscheduler, for gang scheduling

For both cases, following metrics are captured:
1. Scheduling latency
2. Job completion time
3. Dispatch time

### Timestamped events of interest
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

