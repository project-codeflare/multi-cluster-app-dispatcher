# Running Spark Job using AppWrapper and MCAD Controller


### Sources

How-to run Spark in Kubernetes using CLI can be found in https://spark.apache.org/docs/latest/running-on-kubernetes.html#prerequisites 

An example of using Spark Master service and workers can be found in https://github.com/kubernetes/examples/blob/master/staging/spark/README.md 

The following example of running AppWrapper job for Spark is heavily based on the above examples/documentation.

### Creating an AppWrapper job for Spark

An example yaml file for this demo can also be found in 
```/examples/spark-demo-1.yaml```

The content is pasted below, 


```
apiVersion: mcad.ibm.com/v1alpha1
kind: AppWrapper
metadata:
  name: spark-c3-sx
  labels:
    quota_context: "context-3"
    quota_service: "service-x"
spec:
  resources:
    GenericItems:
      - replicas: 1
        generictemplate:
          apiVersion: v1
          kind: ReplicationController
          metadata:
            name: spark-master-controller
          spec:
            replicas: 1
            selector:
              component: spark-master
            template:
              metadata:
                labels:
                  component: spark-master
              spec:
                containers:
                  - name: spark-master
                    image: k8s.gcr.io/spark:1.5.2_v1
                    command: ["/start-master"]
                    ports:
                      - containerPort: 7077
                      - containerPort: 8080
                    resources:
                      requests:
                        cpu: 100m
      - replicas: 1
        generictemplate:
          apiVersion: v1
          kind: ReplicationController
          metadata:
            name: spark-worker-controller
          spec:
            replicas: 3
            selector:
              component: spark-worker
            template:
              metadata:
                labels:
                  component: spark-worker
              spec:
                containers:
                  - name: spark-worker
                    image: k8s.gcr.io/spark:1.5.2_v1
                    command: ["/start-worker"]
                    ports:
                      - containerPort: 8081
                    resources:
                      requests:
                        cpu: 10m
      - replicas: 1
        generictemplate:
          apiVersion: batch/v1
          kind: Job
          metadata:
            name: spark-job
          spec:
            template:
              spec:
                containers:
                  - name: spark-job
                    image: k8s.gcr.io/spark:1.5.2_v1
                    command: ["/bin/bash", "-c"]
                    args: ["/opt/spark/bin/spark-submit --master spark://spark-master:7077 /opt/spark/examples/src/main/python/pi.py"]
                    ports:
                      - containerPort: 8080
                restartPolicy: OnFailure
            backoffLimit: 2
      - replicas: 1
        generictemplate:
          apiVersion: v1
          kind: Service
          metadata:
            name: spark-master
          spec:
            ports:
              - port: 7077
                targetPort: 7077
                name: spark
              - port: 8080
                targetPort: 8080
                name: http
            selector:
              component: spark-master

```

### Running a Spark Job

Once the AppWrapper job is created. It can be used to create pods to run Spark Master Services, Spark Workers and Spark Job. 

```
$ kubectl create -f spark-demo-1.yaml 
appwrapper.mcad.ibm.com/spark-c3-sx created
```

Once created, we can check that the pods are running,

```
$ kubectl get pods
NAME                            READY   STATUS    RESTARTS   AGE
spark-job-vmk64                 1/1     Running   0          8s
spark-master-controller-6lvbv   1/1     Running   0          9s
spark-worker-controller-2lb6m   1/1     Running   0          9s
spark-worker-controller-6x25v   1/1     Running   0          9s
spark-worker-controller-fwf7c   1/1     Running   0          9s
```

and a service has been created, 

```
$ kubectl get services
NAME                TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
. . .
spark-master        ClusterIP   10.96.60.56     <none>        7077/TCP,8080/TCP   16s
. . .
```

As soon as the job is completed, 
```
$ kubectl get pods
NAME                            READY   STATUS      RESTARTS   AGE
spark-job-vmk64                 0/1     Completed   0          24s
spark-master-controller-6lvbv   1/1     Running     0          25s
spark-worker-controller-2lb6m   1/1     Running     0          25s
spark-worker-controller-6x25v   1/1     Running     0          25s
spark-worker-controller-fwf7c   1/1     Running     0          25s
```

The result can be checked with ```kubectl logs``` command as shown below,

```
$ kubectl logs spark-job-vmk64 
...
INFO DAGScheduler: Job 0 finished: reduce at /opt/spark/examples/src/main/python/pi.py:39, took 4.352383 s
Pi is roughly 3.133620
...
```

