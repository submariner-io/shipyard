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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var globalEgressIPGVR = &schema.GroupVersionResource{
	Group:    "submariner.io",
	Version:  "v1",
	Resource: "globalegressips",
}

func globalEgressIPClient(cluster ClusterIndex, namespace string) dynamic.ResourceInterface {
	return DynClients[cluster].Resource(*globalEgressIPGVR).Namespace(namespace)
}

func CreateGlobalEgressIP(cluster ClusterIndex, obj *unstructured.Unstructured) error {
	geipClient := globalEgressIPClient(cluster, obj.GetNamespace())
	AwaitUntil("create GlobalEgressIP", func() (interface{}, error) {
		egressIP, err := geipClient.Create(context.TODO(), obj, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			err = nil
		}

		return egressIP, err
	}, NoopCheckResult)

	return nil
}

func AwaitGlobalEgressIPs(cluster ClusterIndex, name, namespace string) []string {
	gipClient := globalEgressIPClient(cluster, namespace)

	return AwaitAllocatedEgressIPs(gipClient, name)
}
