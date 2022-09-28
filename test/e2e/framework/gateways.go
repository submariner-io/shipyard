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
	"strings"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var gatewayGVR = &schema.GroupVersionResource{
	Group:    "submariner.io",
	Version:  "v1",
	Resource: "gateways",
}

func (f *Framework) AwaitGatewayWithStatus(cluster ClusterIndex, name, status string) *unstructured.Unstructured {
	gwClient := gatewayClient(cluster)
	// Set short name without a suffix as exists in the Gateway resource
	name = strings.Split(name, ".")[0]
	obj := AwaitUntil(fmt.Sprintf("await Gateway on %q with status %q", name, status),
		func() (interface{}, error) {
			resGw, err := gwClient.Get(context.TODO(), name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil, nil // nolint:nilnil // We want to repeat but let the checker known that nothing was found.
			}
			return resGw, err
		},
		func(result interface{}) (bool, string, error) {
			if result == nil {
				return false, "gateway not found yet", nil
			}

			gw := result.(*unstructured.Unstructured)
			haStatus := NestedString(gw.Object, "status", "haStatus")
			if haStatus != status {
				return false, "", fmt.Errorf("gateway %q exists but has wrong status %q, expected %q",
					gw.GetName(), haStatus, status)
			}
			return true, "", nil
		})

	return obj.(*unstructured.Unstructured)
}

func gatewayClient(cluster ClusterIndex) dynamic.ResourceInterface {
	return DynClients[cluster].Resource(*gatewayGVR).Namespace(TestContext.SubmarinerNamespace)
}

func (f *Framework) AwaitGatewaysWithStatus(cluster ClusterIndex, status string) []unstructured.Unstructured {
	gwList := AwaitUntil(fmt.Sprintf("await Gateways with status %q", status),
		func() (interface{}, error) {
			return f.GetGatewaysWithHAStatus(cluster, status), nil
		},
		func(result interface{}) (bool, string, error) {
			gateways := result.([]unstructured.Unstructured)
			if len(gateways) == 0 {
				return false, "no gateway found yet", nil
			}

			return true, "", nil
		})

	return gwList.([]unstructured.Unstructured)
}

func (f *Framework) AwaitGatewayRemoved(cluster ClusterIndex, name string) {
	gwClient := gatewayClient(cluster)

	AwaitUntil(fmt.Sprintf("await Gateway on %q removed", name),
		func() (interface{}, error) {
			_, err := gwClient.Get(context.TODO(), name, metav1.GetOptions{})
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

func (f *Framework) AwaitGatewayFullyConnected(cluster ClusterIndex, name string) *unstructured.Unstructured {
	gwClient := gatewayClient(cluster)
	// Set short name without a suffix as exists in the Gateway resource
	name = strings.Split(name, ".")[0]
	obj := AwaitUntil(fmt.Sprintf("await Gateway on %q with status active and connections UP", name),
		func() (interface{}, error) {
			resGw, err := gwClient.Get(context.TODO(), name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil, nil // nolint:nilnil // We want to repeat but let the checker known that nothing was found.
			}
			return resGw, err
		},
		func(result interface{}) (bool, string, error) {
			if result == nil {
				return false, "gateway not found yet", nil
			}

			gw := result.(*unstructured.Unstructured)
			haStatus := NestedString(gw.Object, "status", "haStatus")
			if haStatus != "active" {
				return false, fmt.Sprintf("Gateway %q exists but not active yet",
					gw.GetName()), nil
			}

			connections, _, _ := unstructured.NestedSlice(gw.Object, "status", "connections")
			if len(connections) == 0 {
				return false, fmt.Sprintf("Gateway %q is active but has no connections yet", name), nil
			}

			for _, o := range connections {
				conn := o.(map[string]interface{})
				status, _, _ := unstructured.NestedString(conn, "status")
				if status != "connected" {
					return false, fmt.Sprintf("Gateway %q is active but cluster %q is not connected: Status: %q, Message: %q",
						name, NestedString(conn, "endpoint", "cluster_id"), status, NestedString(conn, "statusMessage")), nil
				}
			}

			return true, "", nil
		})

	return obj.(*unstructured.Unstructured)
}

func (f *Framework) GetGatewaysWithHAStatus(
	cluster ClusterIndex, status string,
) []unstructured.Unstructured {
	gwClient := gatewayClient(cluster)
	gwList, err := gwClient.List(context.TODO(), metav1.ListOptions{})

	filteredGateways := []unstructured.Unstructured{}

	// List will return "NotFound" if the CRD is not registered in the specific cluster (broker-only)
	if apierrors.IsNotFound(err) {
		return filteredGateways
	}

	Expect(err).NotTo(HaveOccurred())

	for _, gw := range gwList.Items {
		haStatus := NestedString(gw.Object, "status", "haStatus")
		if haStatus == status {
			filteredGateways = append(filteredGateways, gw)
		}
	}

	return filteredGateways
}

func (f *Framework) DeleteGateway(cluster ClusterIndex, name string) {
	AwaitUntil("delete gateway", func() (interface{}, error) {
		err := gatewayClient(cluster).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil, nil // nolint:nilnil // We want to repeat but let the checker known that nothing was found.
		}
		return nil, err
	}, NoopCheckResult)
}

// GatewayCleanup will be executed only on kind environment.
// It will restore the gateway nodes to its initial state.
// Other environments do not need any gw cleanup as MachineSet is responsible to keeping the gw nodes in active states.
func (f *Framework) GatewayCleanup() {
	for cluster := range DynClients {
		passiveGateways := f.FindNodesByGatewayLabel(ClusterIndex(cluster), false)

		for _, nonActiveGw := range passiveGateways {
			if strings.Contains(nonActiveGw.Spec.ProviderID, "kind") {
				By(fmt.Sprintln("Detected passive GW in kind environment - restoring"))
				f.SetGatewayLabelOnNode(ClusterIndex(cluster), nonActiveGw.Name, true)
			}
		}
	}
}
