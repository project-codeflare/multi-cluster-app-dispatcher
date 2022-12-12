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
package v1

import (
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

//AppWrapperLister helps list AppWrappers.
type AppWrapperLister interface {
	// List lists all AppWrappers in the indexer.
	List(selector labels.Selector) (ret []*arbv1.AppWrapper, err error)
	// AppWrappers returns an object that can list and get AppWrappers.
	AppWrappers(namespace string) AppWrapperNamespaceLister
}

// queueJobLister implements the AppWrapperLister interface.
type appWrapperLister struct {
	indexer cache.Indexer
}

//NewAppWrapperLister returns a new AppWrapperLister.
func NewAppWrapperLister(indexer cache.Indexer) AppWrapperLister {
	return &appWrapperLister{indexer: indexer}
}

//List lists all AppWrappers in the indexer.
func (s *appWrapperLister) List(selector labels.Selector) (ret []*arbv1.AppWrapper, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*arbv1.AppWrapper))
	})
	return ret, err
}

//AppWrappers returns an object that can list and get AppWrappers.
func (s *appWrapperLister) AppWrappers(namespace string) AppWrapperNamespaceLister {
	return appWrapperNamespaceLister{indexer: s.indexer, namespace: namespace}
}

//AppWrapperNamespaceLister helps list and get AppWrappers.
type AppWrapperNamespaceLister interface {
	// List lists all AppWrappers in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*arbv1.AppWrapper, err error)
	// Get retrieves the AppWrapper from the indexer for a given namespace and name.
	Get(name string) (*arbv1.AppWrapper, error)
}

// queueJobNamespaceLister implements the AppWrapperNamespaceLister
// interface.
type appWrapperNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all AppWrappers in the indexer for a given namespace.
func (s appWrapperNamespaceLister) List(selector labels.Selector) (ret []*arbv1.AppWrapper, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*arbv1.AppWrapper))
	})
	return ret, err
}

// Get retrieves the AppWrapper from the indexer for a given namespace and name.
func (s appWrapperNamespaceLister) Get(name string) (*arbv1.AppWrapper, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(arbv1.Resource("appwrappers"), name)
	}
	return obj.(*arbv1.AppWrapper), nil
}
