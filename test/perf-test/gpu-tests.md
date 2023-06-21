## MCAD GPU Request Performance test with KWOK
So, I have been doing some performance tests with MCAD using KWOK. The MCAD service for gpu requests is little weird . Here is the experiment I did and what I observed:
1. Created two fake nodes with 8 gpus each by running the script
   ```
   ./nodes.sh
   ```
2. Check that reqested number of nodes have started
  ```
  % kubectl get nodes
  NAME                 STATUS   ROLES           AGE   VERSION
  kind-control-plane   Ready    control-plane   27d   v1.27.1
  kwok-node-1          Ready    agent           7s    fake
  kwok-node-2          Ready    agent           7s    fake
  ```

3. Submit an AW job that wraps two pods, each requesting for 8 gpus.
```
% ./kwokmcadperf.sh 
Checking whether we have a valid cluster login or not...

Nice, looks like you're logged in
Checking MCAD Controller installation status

Nice, MCAD Controller is installed
Checking MCAD Controller installation status

Nice, the KWOK Controller is installed

How many fake KWOK appwrapper jobs do you want? 1
How many pods in a job? 2
How many GPUs do you want to allocate per pod? 8
jobs number is 1
Number of GPUs per pod: 8
Number of pods per AppWrapper: 2
...
...
```

4. We can see that the two pods are scheduled and run to completion.
```
% kubectl get pods                                                 
NAME                                            READY   STATUS      RESTARTS   AGE
fake-defaultaw-schd-spec-with-timeout-1-4r4t2   0/1     Completed   0          4s
fake-defaultaw-schd-spec-with-timeout-1-tx9d5   0/1     Completed   0          4s
```
Furthermore, they are scheduled on two different nodes (as they should be).


5. Delete the previous AW job
```
kubectl delete appwrapper fake-defaultaw-schd-spec-with-timeout-1
```

6. Create a new AW consisting of one pod requesting 16 gpus.
```
% ./kwokmcadperf.sh
Checking whether we have a valid cluster login or not...

Nice, looks like you're logged in
Checking MCAD Controller installation status

Nice, MCAD Controller is installed
Checking MCAD Controller installation status

Nice, the KWOK Controller is installed

How many fake KWOK appwrapper jobs do you want? 1
How many pods in a job? 1
How many GPUs do you want to allocate per pod? 16
jobs number is 1
Number of GPUs per pod: 16
Number of pods per AppWrapper: 1
...
...
```

7. The pod is scheduled to one of the fake nodes (which theoretically it shouldn't).
```
 % kubectl get pods
NAME                                            READY   STATUS      RESTARTS   AGE
fake-defaultaw-schd-spec-with-timeout-1-v7qbk   0/1     Completed   0          40s
```

8. Delete the previous AW job
```
kubectl delete appwrapper fake-defaultaw-schd-spec-with-timeout-1
```

9. Create a new AW consisting of one pod requesting for 24 gpu.
```
% ./kwokmcadperf.sh
Checking whether we have a valid cluster login or not...

Nice, looks like you're logged in
Checking MCAD Controller installation status

Nice, MCAD Controller is installed
Checking MCAD Controller installation status

Nice, the KWOK Controller is installed

How many fake KWOK appwrapper jobs do you want? 1
How many pods in a job? 1
How many GPUs do you want to allocate per pod? 24
jobs number is 1
Number of GPUs per pod: 24
Number of pods per AppWrapper: 1
...
...
```
10. The AW job is now in the queue and is pending.
```
% kubectl describe appwrapper fake-defaultaw-schd-spec-with-timeout-1
...
...
Status:
  Conditions:
    Last Transition Micro Time:  2023-06-21T14:12:40.279735Z
    Last Update Micro Time:      2023-06-21T14:12:40.279734Z
    Status:                      True
    Type:                        Init
    Last Transition Micro Time:  2023-06-21T14:12:40.280678Z
    Last Update Micro Time:      2023-06-21T14:12:40.280677Z
    Reason:                      AwaitingHeadOfLine
    Status:                      True
    Type:                        Queueing
    Last Transition Micro Time:  2023-06-21T14:12:40.289959Z
    Last Update Micro Time:      2023-06-21T14:12:40.289958Z
    Reason:                      FrontOfQueue.
    Status:                      True
    Type:                        HeadOfLine
    Last Transition Micro Time:  2023-06-21T14:12:40.297836Z
    Last Update Micro Time:      2023-06-21T14:12:40.297836Z
    Message:                     Insufficient resources to dispatch AppWrapper.
    Reason:                      AppWrapperNotRunnable.
    Status:                      True
    Type:                        Backoff
  Controllerfirsttimestamp:      2023-06-21T14:12:40.279730Z
  Filterignore:                  true
  Queuejobstate:                 HeadOfLine
  Sender:                        before ScheduleNext - setHOL
  State:                         Pending
  Systempriority:                9
```

11. Add a fake nodes in the cluster with 8 gpus (at this point, the cluster has 24 gpus in total, uniformly spread across 3 nodes)
```
% kubectl apply -f fake-node.yaml 
node/fake-node-1 created
```

12. The job is now dispatched, and runs to completion.
```
% kubectl get pods
NAME                                            READY   STATUS      RESTARTS   AGE
fake-defaultaw-schd-spec-with-timeout-1-fb649   0/1     Completed   0          7s
```

13. This tells us that with respect to KWOK, MCAD is looking at the aggregated gpu resources before making a dispatch decision.
