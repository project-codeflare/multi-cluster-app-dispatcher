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
	v1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/scheme"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type AppWrapperGetter interface {
	AppWrapper(namespaces string) AppWrapperInterface
}

type AppWrapperInterface interface {
	Create(*v1.AppWrapper) (*v1.AppWrapper, error)
	Update(*v1.AppWrapper) (*v1.AppWrapper, error)
	UpdateStatus(*v1.AppWrapper) (*v1.AppWrapper, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.AppWrapper, error)
	List(opts meta_v1.ListOptions) (*v1.AppWrapperList, error)
}

// appwrappers implements AppWrapperInterface
type appwrapper struct {
	client rest.Interface
	ns     string
}

// newAppWrappers returns a AppWrapper
func newAppWrapper(c *ArbV1Client, namespace string) *appwrapper {
	return &appwrapper{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of an appwrapper and creates it.  Returns the server's representation of the appwrapper, and an error, if there is any.
func (c *appwrapper) Create(appwrapper *v1.AppWrapper) (result *v1.AppWrapper, err error) {
	result = &v1.AppWrapper{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		Body(appwrapper).
		Do().
		Into(result)
	return
}

// Update takes the representation of an appwrapper and updates it. Returns the server's representation of the appwrapper, and an error, if there is any.
func (c *appwrapper) Update(appwrapper *v1.AppWrapper) (result *v1.AppWrapper, err error) {
	result = &v1.AppWrapper{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		Name(appwrapper.Name).
		Body(appwrapper).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *appwrapper) UpdateStatus(appwrapper *v1.AppWrapper) (result *v1.AppWrapper, err error) {
	result = &v1.AppWrapper{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		Name(appwrapper.Name).
		SubResource("status").
		Body(appwrapper).
		Do().
		Into(result)
	return
}

// Delete takes name of the appwrapper and deletes it. Returns an error if one occurs.
func (c *appwrapper) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		Name(name).
		Body(options).
		Do().
		Error()
}

// Get takes name of the appwrapper, and returns the corresponding appwrapper object, and an error if there is any.
func (c *appwrapper) Get(name string, options meta_v1.GetOptions) (result *v1.AppWrapper, err error) {
	result = &v1.AppWrapper{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AppWrappers that match those selectors.
func (c *appwrapper) List(opts meta_v1.ListOptions) (result *v1.AppWrapperList, err error) {
	result = &v1.AppWrapperList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource(v1.AppWrapperPlural).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}
