/*
Copyright 2017 The Kubernetes Authors.

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

package v1

import (
	"time"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion/internalinterfaces"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"
)

//AppWrapperInformer provides access to a shared informer and lister for
// AppWrappers.
type AppWrapperInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.AppWrapperLister
}

type appWrapperInformer struct {
	factory internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

//NewAppWrapperInformer constructs a new informer for AppWrapper type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewAppWrapperInformer(client *rest.RESTClient, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredAppWrapperInformer(client, namespace, resyncPeriod, indexers, nil)
}

func NewFilteredAppWrapperInformer(client *rest.RESTClient, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	if(tweakListOptions==nil) {
		source := cache.NewListWatchFromClient(
			client,
			arbv1.AppWrapperPlural,
			namespace,
			fields.Everything())
		return cache.NewSharedIndexInformer(
			source,
			&arbv1.AppWrapper{},
			resyncPeriod,
			indexers,
		)
  } else {
		source := cache.NewFilteredListWatchFromClient(
			client,
			arbv1.AppWrapperPlural,
			namespace,
			tweakListOptions)
		return cache.NewSharedIndexInformer(
			source,
			&arbv1.AppWrapper{},
			resyncPeriod,
			indexers,
		)
	}
}

func (f *appWrapperInformer) defaultAppWrapperInformer(client *rest.RESTClient, resyncPeriod time.Duration) cache.SharedIndexInformer {
  return NewFilteredAppWrapperInformer(client, meta_v1.NamespaceAll, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *appWrapperInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&arbv1.AppWrapper{}, f.defaultAppWrapperInformer)
}

func (f *appWrapperInformer) Lister() v1.AppWrapperLister {
	return v1.NewAppWrapperLister(f.Informer().GetIndexer())
}
