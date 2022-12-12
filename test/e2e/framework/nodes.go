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
	"strconv"
	"strings"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

const (
	gatewayStatusLabel  = "gateway.submariner.io/status"
	gatewayStatusActive = "active"
)

// FindGatewayNodes finds nodes in a given cluster by matching 'submariner.io/gateway' value.
func FindGatewayNodes(cluster ClusterIndex) []v1.Node {
	nodes, err := KubeClients[cluster].CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set{GatewayLabel: "true"}.String(),
	})
	Expect(err).NotTo(HaveOccurred())

	return nodes.Items
}

// FindNonGatewayNodes finds nodes in a given cluster that doesn't match 'submariner.io/gateway' value.
func FindNonGatewayNodes(cluster ClusterIndex) []v1.Node {
	nodes, err := KubeClients[cluster].CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(
			NewRequirement(GatewayLabel, selection.NotEquals, []string{"true"})).String(),
	})
	Expect(err).NotTo(HaveOccurred())

	return nodes.Items
}

// FindClusterWithMultipleGateways finds the cluster with multiple GW nodes.
// Returns cluster index.
func (f *Framework) FindClusterWithMultipleGateways() int {
	for idx := range TestContext.ClusterIDs {
		gatewayNodes := FindGatewayNodes(ClusterIndex(idx))
		if len(gatewayNodes) >= 2 {
			return idx
		}
	}

	return -1
}

// SetGatewayLabelOnNode sets the 'submariner.io/gateway' value for a node to the specified value.
func (f *Framework) SetGatewayLabelOnNode(ctx context.Context, cluster ClusterIndex, nodeName string, isGateway bool) {
	// Escape the '/' char in the label name with the special sequence "~1" so it isn't treated as part of the path
	PatchString("/metadata/labels/"+strings.ReplaceAll(GatewayLabel, "/", "~1"), strconv.FormatBool(isGateway),
		func(pt types.PatchType, payload []byte) error {
			_, err := KubeClients[cluster].CoreV1().Nodes().Patch(ctx, nodeName, pt, payload, metav1.PatchOptions{})
			return err
		})
}
