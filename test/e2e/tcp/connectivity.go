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

// Package tcp implements a TCP/IP connectivity test.
package tcp

import (
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/submariner-io/shipyard/test/e2e/framework"
	v1 "k8s.io/api/core/v1"
	k8snet "k8s.io/utils/net"
)

type EndpointType int

const (
	PodIP EndpointType = iota
	ServiceIP
	// TODO: Remove GlobalIP once all consumer code switches to GlobalServiceIP.
	GlobalIP
	GlobalPodIP
	GlobalServiceIP = GlobalIP
)

type ConnectivityTestParams struct {
	Framework             *framework.Framework
	ConnectionTimeout     uint
	ConnectionAttempts    uint
	Networking            framework.NetworkingType
	FromCluster           framework.ClusterIndex
	FromClusterScheduling framework.NetworkPodScheduling
	ToCluster             framework.ClusterIndex
	ToClusterScheduling   framework.NetworkPodScheduling
	ToEndpointType        EndpointType
	IPFamily              k8snet.IPFamily
}

func (p *ConnectivityTestParams) GetIPFamily() k8snet.IPFamily {
	if p.IPFamily == "" {
		return k8snet.IPv4
	}

	return p.IPFamily
}

func RunConnectivityTest(p *ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {
	if p.ConnectionTimeout == 0 {
		p.ConnectionTimeout = framework.TestContext.ConnectionTimeout
	}

	if p.ConnectionAttempts == 0 {
		p.ConnectionAttempts = framework.TestContext.ConnectionAttempts
	}

	listenerPod, connectorPod := createPods(p)
	listenerPod.CheckSuccessfulFinish()
	connectorPod.CheckSuccessfulFinish()

	framework.By("Verifying that the listener got the connector's data and the connector got the listener's data")
	Expect(listenerPod.TerminationMessage).To(ContainSubstring(connectorPod.Config.Data))
	Expect(connectorPod.TerminationMessage).To(ContainSubstring(listenerPod.Config.Data))

	if p.Networking == framework.PodNetworking && !framework.TestContext.SkipConnectorSrcIPCheck {
		framework.By("Verifying the output of listener pod which must contain the source IP")
		Expect(listenerPod.TerminationMessage).To(ContainSubstring(connectorPod.GetIP()))
	}

	// Return the pods in case further verification is needed
	return listenerPod, connectorPod
}

func RunNoConnectivityTest(p *ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {
	if p.ConnectionTimeout == 0 {
		p.ConnectionTimeout = 5
	}

	if p.ConnectionAttempts == 0 {
		p.ConnectionAttempts = 1
	}

	listenerPod, connectorPod := createPods(p)

	framework.By("Verifying that listener pod exits with non-zero code and timed out message")
	Expect(listenerPod.TerminationMessage).To(ContainSubstring("nc: timeout"))
	Expect(listenerPod.TerminationCode).To(Equal(int32(1)))

	framework.By("Verifying that connector pod exists with zero code but times out")
	Expect(connectorPod.TerminationMessage).To(ContainSubstring("Connection timed out"))
	Expect(connectorPod.TerminationCode).To(Equal(int32(0)))

	// Return the pods in case further verification is needed
	return listenerPod, connectorPod
}

func createPods(p *ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {
	ipFamily := p.GetIPFamily()

	framework.By(fmt.Sprintf("Creating a IPv%v listener pod in cluster %q, which will wait for a handshake over TCP",
		ipFamily, framework.TestContext.ClusterIDs[p.ToCluster]))

	listenerPod := p.Framework.NewNetworkPod(&framework.NetworkPodConfig{
		Type:               framework.ListenerPod,
		Cluster:            p.ToCluster,
		Scheduling:         p.ToClusterScheduling,
		ConnectionTimeout:  p.ConnectionTimeout,
		ConnectionAttempts: p.ConnectionAttempts,
		IsIPv6:             (ipFamily == k8snet.IPv6),
	})

	remoteIP := listenerPod.GetIP()

	if remoteIP == "" {
		framework.Failf("Failed to find matching IP address for IPv%v family on listener pod", p.GetIPFamily())
	}

	var service *v1.Service

	if p.ToEndpointType == ServiceIP {
		framework.By(fmt.Sprintf("Pointing a service ClusterIP to the listener pod in cluster %q",
			framework.TestContext.ClusterIDs[p.ToCluster]))

		service = listenerPod.CreateService()
		remoteIP = service.Spec.ClusterIP
	}

	framework.Logf("Will send traffic to IP: %v", remoteIP)

	framework.By(fmt.Sprintf("Creating a IPv%v connector pod in cluster %q, which will attempt the specific UUID handshake over TCP",
		ipFamily, framework.TestContext.ClusterIDs[p.FromCluster]))

	connectorPod := p.Framework.NewNetworkPod(&framework.NetworkPodConfig{
		Type:               framework.ConnectorPod,
		Cluster:            p.FromCluster,
		Scheduling:         p.FromClusterScheduling,
		RemoteIP:           remoteIP,
		ConnectionTimeout:  p.ConnectionTimeout,
		ConnectionAttempts: p.ConnectionAttempts,
		Networking:         p.Networking,
		IsIPv6:             (ipFamily == k8snet.IPv6),
	})

	framework.By(fmt.Sprintf("Waiting for the connector pod %q to exit, returning what connector sent", connectorPod.Pod.Name))
	connectorPod.AwaitFinish()

	framework.By(fmt.Sprintf("Waiting for the listener pod %q to exit, returning what listener sent", listenerPod.Pod.Name))
	listenerPod.AwaitFinish()

	framework.Logf("Connector pod has IP: %s", connectorPod.GetIP())

	return listenerPod, connectorPod
}
