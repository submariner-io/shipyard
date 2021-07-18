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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var clusterGlobalEgressIPGVR = &schema.GroupVersionResource{
	Group:    "submariner.io",
	Version:  "v1",
	Resource: "clusterglobalegressips",
}

func (f *Framework) AwaitClusterGlobalEgressIPs(cluster ClusterIndex, name string) []string {
	gipClient := clusterGlobalEgressIPClient(cluster)

	return AwaitAllocatedEgressIPs(gipClient, name)
}

func AwaitAllocatedEgressIPs(client dynamic.ResourceInterface, name string) []string {
	obj := AwaitUntil(fmt.Sprintf("await allocated egress IPs for %s", name),
		func() (interface{}, error) {
			resGip, err := client.Get(context.TODO(), name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return resGip, err
		},
		func(result interface{}) (bool, string, error) {
			if result == nil {
				return false, fmt.Sprintf("Egress IP resource %q not found yet", name), nil
			}

			globalIPs := getGlobalIPs(result.(*unstructured.Unstructured))
			if len(globalIPs) == 0 {
				return false, fmt.Sprintf("Egress IP resource %q exists but allocatedIPs not available yet", name), nil
			}
			return true, "", nil
		})

	return getGlobalIPs(obj.(*unstructured.Unstructured))
}

func clusterGlobalEgressIPClient(cluster ClusterIndex) dynamic.ResourceInterface {
	return DynClients[cluster].Resource(*clusterGlobalEgressIPGVR).Namespace(corev1.NamespaceAll)
}

func getGlobalIPs(obj *unstructured.Unstructured) []string {
	if obj != nil {
		globalIPs, _, _ := unstructured.NestedStringSlice(obj.Object, "status", "allocatedIPs")
		return globalIPs
	}
	return []string{}
}
