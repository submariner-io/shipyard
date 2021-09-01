/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

package framework

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	gvr = schema.GroupVersionResource{
		Group:    "multicluster.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "serviceexports",
	}
)

func (f *Framework) CreateServiceExport(cluster ClusterIndex, name string) {
	resourceServiceExport := &unstructured.Unstructured{}
	resourceServiceExport.SetName(name)
	resourceServiceExport.SetNamespace(f.Namespace)
	resourceServiceExport.SetKind("ServiceExport")
	resourceServiceExport.SetAPIVersion("multicluster.x-k8s.io/v1alpha1")

	svcExs := DynClients[cluster].Resource(gvr).Namespace(f.Namespace)

	_ = AwaitUntil("create service export", func() (interface{}, error) {
		result, err := svcExs.Create(context.TODO(), resourceServiceExport, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			err = nil
		}
		return result, err
	}, NoopCheckResult).(*unstructured.Unstructured)
}

func (f *Framework) DeleteServiceExport(cluster ClusterIndex, name string) {
	AwaitUntil("delete service export", func() (interface{}, error) {
		return nil, DynClients[cluster].Resource(gvr).Namespace(f.Namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}, NoopCheckResult)
}
