## Dispatch performance of MCAD with large number of small jobs
The following experiment was performed to understand the overhead associated with MCAD, if any, when large number of small AW jobs, consisting ony of 1 pod, are inserted into a system.

We first design a baseline experiment where only default kubernetes scheduler without MCAD was used. A homogenous cluster of 1000 nodes was deloyed.

Deploy 1000 fake nodes
```
./nodes.sh
```

Check the number of desired nodes in the cluster by 
```
kubectl get nodes --selector type=kwok -o name | wc -l
```

1. **Without MCAD:** Deploy fake job without using AppWrapper/MCAD using `nomcadkwokperf.sh` script. The script takes three arguments: `-j` for the number of jobs to be deployed, `-a` for the number of pods in a job, `-g` for the number of gpus per pod. Since this is a baseline experiment for small jobs, I set `-a 1` and `-g 0`.  The script `nomcadkwokperf.sh` gives the total time it took to complete `j` number of jobs since the first job was submitted and one can play with parameter `-j` to understand the rate of completion with respect to different load. In order to average over multiple runs of the script, run 

```
./nomcad-sim-run.sh -j 100 -g 0 -a 1
```
`nomcad-sim-run.sh` iterates `nomcadkwokperf.sh` 10 times, and get the average completion rate per iteration. 

2. **With MCAD:** The script runs are similar to aforementioned experiment, except the script names. `mcadkwokperf.sh` script deploys jobs as AppWrapper jobs. Furthermore, to calculate the average completion time, run 
```
./mcad-sim-run.sh -j 100 -g 0 -a 1
```


Some baseline results for this experiment can be found [here](https://ibm.box.com/s/glxzte7g4rtk1ew0i7gc17kxugyqn59j)