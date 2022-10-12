### Kuberay-MCAD integration

This integration will help in queuing on [kuberay](https://github.com/ray-project/kuberay) clusters in Multi-cluster-app-dispatcher (MCAD) until aggregated resources are available in the cluster.

#### Prerequisites

- kubernetes or Openshift cluster 
- Install MCAD using instructions present under `deployment` directory
- Make sure MCAD has clusterrole to create ray resources, please patch using configuration file present in `config` directory with name `xqueuejob-controller.yaml`

#### Steps

- Install kuberay operator from [link](https://docs.ray.io/en/latest/cluster/kubernetes/getting-started.html#deploying-the-kuberay-operator)
- Submit ray cluster to MCAD as appwrapper using the config file `aw-raycluster.yaml` present in the `config` directory using command `kubectl create -f aw-raycluster.yaml`
- Check the status of the appwrapper using command `kubectl describe appwrapper <your-appwrapper-name>`
- Check running pods using command `kubectl get pods -n <your-name-space>`