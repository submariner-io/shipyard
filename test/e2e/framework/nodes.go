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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// FindNodesByGatewayLabel finds the nodes in a given cluster by matching 'submariner.io/gateway' value.
// Nodes with the missing label will be ignored. Note the control plane node labeled as master is ignored.
func (f *Framework) FindNodesByGatewayLabel(cluster ClusterIndex, isGateway bool) []*v1.Node {
	return findNodesByGatewayLabel(int(cluster), isGateway)
}

func findNodesByGatewayLabel(cluster int, isGateway bool) []*v1.Node {
	nodes := AwaitUntil("list nodes", func() (interface{}, error) {
		// Ignore the control plane node labeled as master as it doesn't allow scheduling of pods
		return KubeClients[cluster].CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "!node-role.kubernetes.io/master",
		})
	}, NoopCheckResult).(*v1.NodeList)

	expLabelValue := strconv.FormatBool(isGateway)
	retNodes := []*v1.Node{}

	for i := range nodes.Items {
		value, exists := nodes.Items[i].Labels[GatewayLabel]
		if !exists {
			continue
		}

		if value == expLabelValue {
			retNodes = append(retNodes, &nodes.Items[i])
		}
	}

	return retNodes
}

// SetGatewayLabelOnNode sets the 'submariner.io/gateway' value for a node to the specified value.
func (f *Framework) SetGatewayLabelOnNode(cluster ClusterIndex, nodeName string, isGateway bool) {
	// Escape the '/' char in the label name with the special sequence "~1" so it isn't treated as part of the path
	PatchString("/metadata/labels/"+strings.ReplaceAll(GatewayLabel, "/", "~1"), strconv.FormatBool(isGateway),
		func(pt types.PatchType, payload []byte) error {
			_, err := KubeClients[cluster].CoreV1().Nodes().Patch(context.TODO(), nodeName, pt, payload, metav1.PatchOptions{})
			return err
		})
}

// FindAnyNonGatewayRouteAgentPodNodes looks for the route agent pod on any cluster node, including control plane
// The function will be used in upstream ci environment, lack of resources where two GW nodes are used.
func (f *Framework) FindAnyNonGatewayRouteAgentPodNodes(cluster ClusterIndex) []*v1.Node {
	gwNodesSet := make(map[string]bool)
	gwNodes := f.FindNodesByGatewayLabel(cluster, true)

	for _, node := range gwNodes {
		gwNodesSet[node.Name] = true
	}

	var selectedNode string
	routeAgentPods := f.AwaitPodsByAppLabel(cluster, RouteAgent, TestContext.SubmarinerNamespace, -1)

	for i := range routeAgentPods.Items {
		if !gwNodesSet[routeAgentPods.Items[i].Spec.NodeName] {
			selectedNode = routeAgentPods.Items[i].Spec.NodeName
		}
	}

	searchNodes := AwaitUntil("list nodes", func() (interface{}, error) {
		// Ignore the control plane node labeled as master as it doesn't allow scheduling of pods
		return KubeClients[cluster].CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "kubernetes.io/hostname=" + selectedNode,
		})
	}, NoopCheckResult).(*v1.NodeList)

	nodes := []*v1.Node{}
	for i := range searchNodes.Items {
		nodes = append(nodes, &searchNodes.Items[i])
	}

	return nodes
}
