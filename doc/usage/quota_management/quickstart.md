## Intro

Quota trees are a concept that allows users to define and enforce resource limits for arbitrary kubernetes objects based
on a hierarchical structure of quotas. A quota tree consists of a root node and several child nodes, each representing a
quota. A quota is a set of rules that specify the maximum amount of resources that a kubernetes object can use. The root
node of a quota tree defines the global limits for the entire tree. The child nodes can have their own limits, but they
cannot exceed the limits of their parent node. For example, if the root node has a limit of 2 CPU and 4 GiB of memory,
then any child node cannot have more than 2 CPU and 4 GiB of memory.

One of the key features of quota trees is the hardLimit attribute, which determines whether a quota can borrow resources
from its siblings or not. A sibling is another child node that shares the same parent node. If a quota has hardLimit set
to true, then it can only use the resources defined in its specification. If a quota has hardLimit set to false, then it
can use any unused resources from its siblings, as long as it does not violate the parent node’s limits. However, if an
object is defined which neccesitates borrowing from a sibling node, then if another kubernetes object is created which
uses the borrowees quota (and is within the borrowees limit) then the borrower will be preempted to free these
resources.

Quota trees are useful for managing and partitioning resources within a kubernetes cluster. They can help users to
optimize resource utilization, avoid resource starvation, and ensure quality of service for different types of objects.

## Example QuotaSubtree

```yaml
apiVersion: quota.codeflare.dev/v1
kind: QuotaSubtree
metadata:
  name: context-root
  namespace: kube-system  # this is the namespace where the multi-cluster-app-dispatcher controller lives
  labels:
    tree: quota_context  # the tree to which all nodes defined in this QuotaSubtree object belong
spec:
  children:
    - name: context-root
      quotas:
        hardLimit: true  # if context-root has siblings it cannot borrow from them
        requests:
          cpu: 2000m
          memory: 8000Mi
---
apiVersion: quota.codeflare.dev/v1
kind: QuotaSubtree
metadata:
  name: context-root-children
  namespace: kube-system
  labels:
    tree: quota_context  # note that this label is the same as above indicating they belong to the same tree
spec:
  parent: context-root
  children:
    - name: alpha
      quotas:
        hardLimit: false  # objects using 'alpha' can borrow resources from beta (this is the default value)
        requests:
          cpu: 1000m  # even though beta has 2000m 'alpha' will only be able to borrow 1000m due to the limit imposed by context root
          memory: 4000Mi
    - name: beta
      quotas:
        hardLimit: true  # objects using 'beta' cannot borrow extra resources from 'alpha'
        requests:
          cpu: 2000m
          memory: 4000Mi
```

## Using quota trees in your AppWrappers

```yaml
apiVersion: workload.codeflare.dev/v1beta1
kind: AppWrapper
metadata:
  name: myGangScheduledApp
  namespace: default
  labels:
    quota_context: "alpha"  # this label indicates the tree and the node in that tree "tree_name: node_name"
spec:
  service:
    spec: {}
  resources:
    metadata: {}
    GenericItems:
      - metadata: {}
        replicas: 1
        custompodresources:  # because AppWrappers are generic they must define the resultant pods that will be needed
                             # to fulfill a request as the request values cannot be reliably extracted from the
                             # generictemplate below
        - replicas:  1
          requests:
            cpu: 1000m
            memory: 4000Mi
          limits:
            cpu: 1000m
            memory: 4000Mi
        generictemplate:  # the k8 object that is deployed once the resource requirements are met, this can be any native 
                          # k8 objects or a custom resource
          apiVersion: apps/v1
          kind: StatefulSet
          metadata:
            name: myGangScheduledApp
            namespace: test1
            labels:
              app: myGangScheduledApp
          spec:
            selector:
              matchLabels:
                app: myGangScheduledApp
            replicas: 1
            template:
              metadata:
                labels:
                  app: myGangScheduledApp
                  size: "1"
              spec:
                containers:
                  - name: myGangScheduledApp
                    image: registry.access.redhat.com/ubi8/ubi:latest
                    command:
                      - /bin/sh
                      - -c
                      - while true; do sleep 10; done
                    resources:
                      requests:
                        cpu: "1000m"
                        memory: "4000Mi"
                      limits:
                        cpu: "1000m"
                        memory: "4000Mi"
```