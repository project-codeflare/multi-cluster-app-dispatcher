/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queuejobdispatch

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/cache"

	clientset "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned"

	informerFactory "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions"
	arbinformers "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions/controller/v1beta1"
	arblisters "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1beta1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type JobClusterAgent struct {
	AgentId         string
	DeploymentName  string
	queuejobclients *clientset.Clientset
	k8sClients      *kubernetes.Clientset // for the update of aggr resouces
	AggrResources   *clusterstateapi.Resource

	jobInformer arbinformers.AppWrapperInformer
	jobLister   arblisters.AppWrapperLister
	jobSynced   func() bool

	agentEventQueue *cache.FIFO
}

func NewJobClusterAgent(config string, agentEventQueue *cache.FIFO) *JobClusterAgent {
	configStrings := strings.Split(config, ":")
	if len(configStrings) < 2 {
		klog.Errorf("[agentEventQueue] Invalid agent configuration: %s.  Agent cluster will not be instantiated.", config)
		return nil
	}
	klog.V(2).Infof("[Dispatcher: Agent] Creation: %s\n", "/root/kubernetes/"+configStrings[0])

	agent_config, err := clientcmd.BuildConfigFromFlags("", "/root/kubernetes/"+configStrings[0])
	if err != nil {
		klog.V(2).Infof("[Dispatcher: Agent] Cannot create client\n")
		return nil
	}
	qa := &JobClusterAgent{
		AgentId:         "/root/kubernetes/" + configStrings[0],
		DeploymentName:  configStrings[1],
		queuejobclients: clientset.NewForConfigOrDie(agent_config),
		k8sClients:      kubernetes.NewForConfigOrDie(agent_config),
		AggrResources:   clusterstateapi.EmptyResource(),
	}
	qa.agentEventQueue = agentEventQueue

	if qa.queuejobclients == nil {
		klog.V(2).Infof("[Dispatcher: Agent] Cannot Create Client\n")
	} else {
		klog.V(2).Infof("[Dispatcher: Agent] %s: Create Clients Suceessfully\n", qa.AgentId)
	}

	appWrapperClient, err := clientset.NewForConfig(agent_config)
	if err != nil {
		klog.Fatalf("Could not instantiate k8s client, err=%v", err)
	}

	qa.jobInformer = informerFactory.NewFilteredSharedInformerFactory(appWrapperClient, 0, v1.NamespaceAll,
		func(opt *metav1.ListOptions) {
			opt.LabelSelector = "IsDispatched=true"
		},
	).Mcad().V1beta1().AppWrappers()
	qa.jobInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *arbv1.AppWrapper:
					klog.V(4).Infof("Filter AppWrapper name(%s) namespace(%s)\n", t.Name, t.Namespace)
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qa.addQueueJob,
				UpdateFunc: qa.updateQueueJob,
				DeleteFunc: qa.deleteQueueJob,
			},
		})

	qa.jobLister = qa.jobInformer.Lister()

	qa.jobSynced = qa.jobInformer.Informer().HasSynced

	qa.UpdateAggrResources(context.Background())

	return qa
}

func (cc *JobClusterAgent) addQueueJob(obj interface{}) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("obj is not AppWrapper")
		return
	}
	klog.V(10).Infof("[TTime]: %s Adding New Job: %s to EventQ\n", time.Now().String(), qj.Name)
	cc.agentEventQueue.Add(qj)
}

func (cc *JobClusterAgent) updateQueueJob(oldObj, newObj interface{}) {
	newQJ, ok := newObj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("newObj is not AppWrapper")
		return
	}
	klog.V(10).Infof("[TTime]: %s Adding Update Job: %s to EventQ\n", time.Now().String(), newQJ.Name)
	cc.agentEventQueue.Add(newQJ)
}

func (cc *JobClusterAgent) deleteQueueJob(obj interface{}) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("obj is not AppWrapper")
		return
	}
	klog.V(10).Infof("[TTime]: %s Adding Delete Job: %s to EventQ\n", time.Now().String(), qj.Name)
	cc.agentEventQueue.Add(qj)
}

func (qa *JobClusterAgent) Run(stopCh <-chan struct{}) {
	go qa.jobInformer.Informer().Run(stopCh)
	cache.WaitForCacheSync(stopCh, qa.jobSynced)
}

func (qa *JobClusterAgent) DeleteJob(ctx context.Context, cqj *arbv1.AppWrapper) {
	qj_temp := cqj.DeepCopy()
	klog.V(2).Infof("[Dispatcher: Agent] Request deletion of XQJ %s to Agent %s\n", qj_temp.Name, qa.AgentId)
	qa.queuejobclients.McadV1beta1().AppWrappers(qj_temp.Namespace).Delete(ctx, qj_temp.Name, metav1.DeleteOptions{})
}

func (qa *JobClusterAgent) CreateJob(ctx context.Context, cqj *arbv1.AppWrapper) {
	qj_temp := cqj.DeepCopy()
	agent_qj := &arbv1.AppWrapper{
		TypeMeta:   qj_temp.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: qj_temp.Name, Namespace: qj_temp.Namespace},
		Spec:       qj_temp.Spec,
	}
	agent_qj.Status.CanRun = qj_temp.Status.CanRun
	agent_qj.Status.IsDispatched = qj_temp.Status.IsDispatched

	if agent_qj.Labels == nil {
		agent_qj.Labels = map[string]string{}
	}
	for k, v := range qj_temp.Labels {
		agent_qj.Labels[k] = v
	}
	agent_qj.Labels["IsDispatched"] = "true"

	klog.V(2).Infof("[Dispatcher: Agent] Create XQJ: %s (Status: %+v) in Agent %s\n", agent_qj.Name, agent_qj.Status, qa.AgentId)
	qa.queuejobclients.McadV1beta1().AppWrappers(agent_qj.Namespace).Create(ctx, agent_qj, metav1.CreateOptions{})
}

type ClusterMetricsList struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink string `json:"selfLink"`
	} `json:"metadata"`
	Items []struct {
		MetricName   string            `json:"metricName"`
		MetricLabels map[string]string `json:"metriclabels"`
		Timestamp    string            `json:"timestamp"`
		Value        string            `json:"value"`
	} `json:"items"`
}

func (qa *JobClusterAgent) UpdateAggrResources(ctx context.Context) error {
	klog.V(6).Infof("[Dispatcher: UpdateAggrResources] Getting aggregated resources for Agent ID: %s with Agent Name: %s\n", qa.AgentId, qa.DeploymentName)

	// Read the Agent XQJ Deployment object
	if qa.k8sClients == nil {
		return nil

	}

	data, err := qa.k8sClients.RESTClient().Get().AbsPath("apis/external.metrics.k8s.io/v1beta1/namespaces/default/cluster-external-metric").DoRaw(ctx)

	if err != nil {
		klog.V(2).Infof("[Dispatcher: UpdateAggrResources] Failed to get metrics from deployment Agent ID: %s with Agent Name: %s, Error: %v\n", qa.AgentId, qa.DeploymentName, err)

	} else {
		res := &ClusterMetricsList{}
		unmarshalerr := json.Unmarshal(data, res)
		if unmarshalerr != nil {
			klog.V(2).Infof("[Dispatcher: UpdateAggrResources] Failed to unmarshal metrics to struct: %v from deployment Agent ID: %s with Agent Name: %s, Error: %v\n",
				res, qa.AgentId, qa.DeploymentName, unmarshalerr)
		} else {
			if len(res.Items) > 0 {
				for i := 0; i < len(res.Items); i++ {
					klog.V(9).Infof("[Dispatcher: UpdateAggrResources] Obtained the metric:%s, label:%v, value: %s, from the Agent: %s  with Agent Name: %s.\n",
						res.Items[i].MetricName, res.Items[i].MetricLabels, res.Items[i].Value, qa.AgentId, qa.DeploymentName)
					clusterMetricType := res.Items[i].MetricLabels["cluster"]

					if strings.Compare(clusterMetricType, "cpu") == 0 || strings.Compare(clusterMetricType, "memory") == 0 {
						val, units, _ := getFloatString(res.Items[i].Value)
						num, err := strconv.ParseFloat(val, 64)
						if err != nil {
							klog.Warningf("[Dispatcher: UpdateAggrResources] Possible issue converting %s string value of %s due to error: %v\n",
								clusterMetricType, res.Items[i].Value, err)
						} else {
							if units == "k" {
								num = num * 1000
							}
							f_num := math.Float64bits(num)
							f_zero := math.Float64bits(0.0)
							if f_num > f_zero {
								if strings.Compare(clusterMetricType, "cpu") == 0 {
									qa.AggrResources.MilliCPU = num
									klog.V(10).Infof("[Dispatcher: UpdateAggrResources] Updated %s from %f to %f for metrics: %v from deployment Agent ID: %s with Agent Name: %s\n",
										clusterMetricType, qa.AggrResources.MilliCPU, num, res, qa.AgentId, qa.DeploymentName)
								} else {
									qa.AggrResources.Memory = num
									klog.V(10).Infof("[Dispatcher: UpdateAggrResources] Updated %s from %f to %f for metrics: %v from deployment Agent ID: %s with Agent Name: %s\n",
										clusterMetricType, qa.AggrResources.Memory, num, res, qa.AgentId, qa.DeploymentName)
								}
							} else {
								klog.Warningf("[Dispatcher: UpdateAggrResources] Possible issue converting %s string value of %s to float type.  Conversion result: %f\n",
									clusterMetricType, res.Items[i].Value, num)
							} // Float value resulted in zero value.
						} // Converting string to float success
					} else if strings.Compare(clusterMetricType, "gpu") == 0 {
						num, err := getInt64String(res.Items[i].Value)
						if err != nil {
							klog.Warningf("[Dispatcher: UpdateAggrResources] Possible issue converting %s string value of %s due to int64 type, error: %v\n",
								clusterMetricType, res.Items[i].Value, err)
						} else {
							qa.AggrResources.GPU = num
						}
					} else {
						klog.V(9).Infof("[Dispatcher: UpdateAggrResources] Unknown label value: %s for metrics: %v from deployment Agent ID: %s with Agent Name: %s\n",
							clusterMetricType, res, qa.AgentId, qa.DeploymentName)
					} // Unknown label

				}
			} else {
				klog.V(2).Infof("[Dispatcher: UpdateAggrResources] Failed to obtain values for metrics: %v from deployment Agent ID: %s with Agent Name: %s, Error: %v\n", res, qa.AgentId, qa.DeploymentName, unmarshalerr)
			}
		}
	}

	klog.V(4).Infof("[Dispatcher: UpdateAggrResources] Updated Aggr Resources of %s: %v\n", qa.AgentId, qa.AggrResources)
	return nil
}

func getFloatString(num string) (string, string, error) {
	var validatedNum string = num
	var numUnits string = ""
	_, err := strconv.ParseFloat(validatedNum, 64)
	if err != nil {
		if strings.HasSuffix(validatedNum, "k") {
			validatedNum = validatedNum[:len(validatedNum)-len("k")]
		}
		_, err := strconv.ParseFloat(validatedNum, 64)
		if err == nil {
			numUnits = "k"
		} else {
			validatedNum = num
		}
	} else {
		validatedNum = num
	}
	return validatedNum, numUnits, err
}
func getInt64String(num string) (int64, error) {
	var validatedNum int64 = 0
	n, err := strconv.ParseInt(num, 10, 64)
	if err == nil {
		validatedNum = n
	}
	return validatedNum, err
}
