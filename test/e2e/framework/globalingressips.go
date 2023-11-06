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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var globalIngressIPGVR = &schema.GroupVersionResource{
	Group:    "submariner.io",
	Version:  "v1",
	Resource: "globalingressips",
}

func (f *Framework) AwaitGlobalIngressIP(cluster ClusterIndex, name, namespace string) string {
	if TestContext.GlobalnetEnabled {
		gipClient := globalIngressIPClient(cluster, namespace)
		obj := AwaitUntil(fmt.Sprintf("await GlobalIngressIP %s/%s", namespace, name),
			func() (interface{}, error) {
				resGip, err := gipClient.Get(context.TODO(), name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return nil, nil //nolint:nilnil // We want to repeat but let the checker known that nothing was found.
				}
				return resGip, err
			},
			func(result interface{}) (bool, string, error) {
				if result == nil {
					return false, fmt.Sprintf("GlobalEgressIP %s not found yet", name), nil
				}

				globalIP := getGlobalIP(result.(*unstructured.Unstructured))
				if globalIP == "" {
					return false, fmt.Sprintf("GlobalIngress %q exists but allocatedIP not available yet",
						name), nil
				}
				return true, "", nil
			})

		return getGlobalIP(obj.(*unstructured.Unstructured))
	}

	return ""
}

func (f *Framework) AwaitGlobalIngressIPRemoved(cluster ClusterIndex, name, namespace string) {
	gipClient := globalIngressIPClient(cluster, namespace)
	AwaitUntil(fmt.Sprintf("await GlobalIngressIP %s/%s removed", namespace, name),
		func() (interface{}, error) {
			_, err := gipClient.Get(context.TODO(), name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		},
		func(result interface{}) (bool, string, error) {
			gone := result.(bool)
			return gone, "", nil
		})
}

func globalIngressIPClient(cluster ClusterIndex, namespace string) dynamic.ResourceInterface {
	return DynClients[cluster].Resource(*globalIngressIPGVR).Namespace(namespace)
}

func getGlobalIP(obj *unstructured.Unstructured) string {
	if obj != nil {
		globalIP, _, _ := unstructured.NestedString(obj.Object, "status", "allocatedIP")
		return globalIP
	}

	return ""
}
