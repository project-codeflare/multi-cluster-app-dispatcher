package e2e

import (
	//gcontext "context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetes "k8s.io/client-go/kubernetes"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func connectToK8s() *kubernetes.Clientset {
	home, exists := os.LookupEnv("HOME")
	if !exists {
		home = "/root"
	}

	configPath := filepath.Join(home, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		log.Fatalln("failed to create K8s config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln("Failed to create K8s clientset")
	}

	return clientset
}

/* func launchK8sJob(context *context, jobName *string, image *string, cmd *string) error {
	jobs := context.kubeclient.BatchV1().Jobs("default")
	var backOffLimit int32 = 0

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *jobName,
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    *jobName,
							Image:   *image,
							Command: strings.Split(*cmd, " "),
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
			BackoffLimit: &backOffLimit,
		},
	}

	_, err := jobs.Create(gcontext.Background(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		log.Fatalln("Failed to create K8s job.")
	}

	//print job details
	log.Println("Created K8s job successfully")
	return err
}

func launchK8sStatefulSet(context *context, jobName *string, image *string, cmd *string) error {
	//jobs := context.kubeclient.BatchV1().Jobs("default")
	ssets := context.kubeclient.AppsV1().StatefulSets("default")
	//claims := []v1.PersistentVolumeClaim{}
	//var backOffLimit int32 = 0
	template := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
				},
			},
		},
	}

	template.Labels = map[string]string{"foo": "bar"}
	ssSpec := &apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      *jobName,
			Namespace: v1.NamespaceDefault,
			UID:       types.UID("test"),
		},
		Spec: apps.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Replicas:       func() *int32 { i := int32(1); return &i }(),
			Template:       template,
			ServiceName:    "governingsvc",
			UpdateStrategy: apps.StatefulSetUpdateStrategy{Type: apps.RollingUpdateStatefulSetStrategyType},
			RevisionHistoryLimit: func() *int32 {
				limit := int32(2)
				return &limit
			}(),
		},
	}

	_, err := ssets.Create(gcontext.Background(), ssSpec, metav1.CreateOptions{})
	if err != nil {
		log.Fatalln("Failed to create StatefulSet.")
	}

	//print job details
	log.Println("Created K8s StatefulSet successfully")
	return err
} */

func launchK8sAppWrapper(context *context, jobName string) (*arbv1.AppWrapper, error) {
	rb := []byte(`{"apiVersion": "apps/v1",
		"kind": "StatefulSet", 
	"metadata": {
		"name": "aw-statefulset-3",
		"namespace": "test",
		"labels": {
			"app": "aw-statefulset-3"
		}
	},
	"spec": {
		"replicas": 2,
		"selector": {
			"matchLabels": {
				"app": "aw-statefulset-3"
			}
		},
		"template": {
			"metadata": {
				"labels": {
					"app": "aw-statefulset-3"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "aw-statefulset-3",
						"image": "k8s.gcr.io/echoserver:1.4",
						"imagePullPolicy": "Never",
						"ports": [
							{
								"containerPort": 80
							}
						]
					}
				]
			}
		}
	}} `)
	var schedSpecMin int = 2

	aw := &arbv1.AppWrapper{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "test",
		},
		Spec: arbv1.AppWrapperSpec{
			SchedSpec: arbv1.SchedulingSpecTemplate{
				MinAvailable: schedSpecMin,
			},
			AggrResources: arbv1.AppWrapperResourceList{
				Items: []arbv1.AppWrapperResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("%s-%s", jobName, "item1"),
							Namespace: "test",
						},
						Replicas: 1,
						Type:     arbv1.ResourceTypeStatefulSet,
						Template: runtime.RawExtension{
							Raw: rb,
						},
					},
				},
			},
		},
	}
	fmt.Println("Printing though fmt")
	klog.Info("Printing...")
	res2B, _ := json.Marshal(aw)
	fmt.Println(string(res2B))
	appwrapper, err := context.karclient.ArbV1().AppWrappers(context.namespace).Create(aw)
	return appwrapper, err

}
