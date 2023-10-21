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

func findGateway(cluster ClusterIndex, name string) (*unstructured.Unstructured, error) {
	gwClient := gatewayClient(cluster)
	resGw, err := gwClient.Get(context.TODO(), name, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		// Some environments sets a node in Gateway resource without a suffix
		resGw, err = gwClient.Get(context.TODO(), strings.Split(name, ".")[0], metav1.GetOptions{})
	}

	if apierrors.IsNotFound(err) {
		return nil, nil //nolint:nilnil // We want to repeat but let the checker known that nothing was found.
	}

	return resGw, err
}

func (f *Framework) AwaitGatewayWithStatus(cluster ClusterIndex, name, status string) *unstructured.Unstructured {
	obj := AwaitUntil(fmt.Sprintf("await Gateway on %q with status %q", name, status),
		func() (interface{}, error) {
			return findGateway(cluster, name)
		},
		func(result interface{}) (bool, string, error) {
			if result == nil {
				return false, "gateway not found yet", nil
			}

			gw := result.(*unstructured.Unstructured)
			haStatus := NestedString(gw.Object, "status", "haStatus")
			if haStatus != status {
				return false, fmt.Sprintf("gateway %q exists but has wrong status %q, expected %q", gw.GetName(), haStatus, status), nil
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
	obj := AwaitUntil(fmt.Sprintf("await Gateway on %q with status active and connections UP", name),
		func() (interface{}, error) {
			return findGateway(cluster, name)
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
			return nil, nil //nolint:nilnil // We want to repeat but let the checker known that nothing was found.
		}
		return nil, err
	}, NoopCheckResult)
}

func (f *Framework) SaveGatewayNode(cluster ClusterIndex, gwNode string) {
	f.gatewayNodesToReset[int(cluster)] = append(f.gatewayNodesToReset[int(cluster)], gwNode)
}

// GatewayCleanup will be executed only on kind environment.
// It will restore the gateway nodes to its initial state.
// Other environments do not need any gw cleanup as MachineSet is responsible to keeping the gw nodes in active states.
func (f *Framework) GatewayCleanup() {
	ctx := context.TODO()

	for cluster := range f.gatewayNodesToReset {
		for _, gnode := range f.gatewayNodesToReset[cluster] {
			By(fmt.Sprintf("Restoring gateway %q on cluster %q", gnode, TestContext.ClusterIDs[cluster]))
			f.SetGatewayLabelOnNode(ctx, ClusterIndex(cluster), gnode, true)
		}
	}
}

// Perform a gateway failover.
// The failover for the real environment will crash the gateway node.
// The failover for the kind environment will set the submariner.io/gateway label to "false" on the gw node.
func (f *Framework) DoFailover(ctx context.Context, cluster ClusterIndex, gwNode, gwPod string) {
	provider := DetectProvider(ctx, cluster, gwNode)

	if provider == "kind" {
		f.SaveGatewayNode(cluster, gwNode)
		f.SetGatewayLabelOnNode(ctx, cluster, gwNode, false)
	} else {
		cmd := []string{"sh", "-c", "echo 1 > /proc/sys/kernel/sysrq && echo b > /proc/sysrq-trigger"}

		_, _, err := f.ExecWithOptions(ctx, &ExecOptions{
			Command:       cmd,
			Namespace:     TestContext.SubmarinerNamespace,
			PodName:       gwPod,
			ContainerName: SubmarinerGateway,
			CaptureStdout: false,
			CaptureStderr: true,
		}, cluster)
		if err != nil {
			if strings.Contains(err.Error(), "unable to upgrade connection: container not found") {
				By(fmt.Sprintf("Successfully crashed gateway node %q", gwNode))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}
}
