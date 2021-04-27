/*
© 2020 Red Hat, Inc.

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
package tcp

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/shipyard/test/e2e/framework"
	v1 "k8s.io/api/core/v1"
)

const globalnetGlobalIPAnnotation = "submariner.io/globalIp"

type EndpointType int

const (
	PodIP EndpointType = iota
	ServiceIP
	GlobalIP
)

type ConnectivityTestParams struct {
	Framework             *framework.Framework
	Networking            framework.NetworkingType
	ConnectionTimeout     uint
	ConnectionAttempts    uint
	FromCluster           framework.ClusterIndex
	FromClusterScheduling framework.NetworkPodScheduling
	ToCluster             framework.ClusterIndex
	ToClusterScheduling   framework.NetworkPodScheduling
	ToEndpointType        EndpointType
}

func RunConnectivityTest(p ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {
	if p.ConnectionTimeout == 0 {
		p.ConnectionTimeout = framework.TestContext.ConnectionTimeout
	}

	if p.ConnectionAttempts == 0 {
		p.ConnectionAttempts = framework.TestContext.ConnectionAttempts
	}

	listenerPod, connectorPod := createPods(&p)
	listenerPod.CheckSuccessfulFinish()
	connectorPod.CheckSuccessfulFinish()

	By("Verifying that the listener got the connector's data and the connector got the listener's data")
	Expect(listenerPod.TerminationMessage).To(ContainSubstring(connectorPod.Config.Data))
	Expect(connectorPod.TerminationMessage).To(ContainSubstring(listenerPod.Config.Data))

	if p.Networking == framework.PodNetworking {
		if p.ToEndpointType == GlobalIP {
			// When Globalnet is enabled (i.e., remoteEndpoint is a globalIP) and POD uses PodNetworking,
			// Globalnet Controller MASQUERADEs the source-ip of the POD to the corresponding global-ip
			// that is assigned to the POD.
			By("Verifying the output of listener pod which must contain the globalIP of the connector POD")
			podGlobalIP := connectorPod.Pod.GetAnnotations()[globalnetGlobalIPAnnotation]
			Expect(podGlobalIP).ToNot(Equal(""))
			Expect(listenerPod.TerminationMessage).To(ContainSubstring(podGlobalIP))
		} else if p.ToEndpointType != GlobalIP {
			By("Verifying the output of listener pod which must contain the source IP")
			Expect(listenerPod.TerminationMessage).To(ContainSubstring(connectorPod.Pod.Status.PodIP))
		}
	} else if p.Networking == framework.HostNetworking {
		// when a POD is using the HostNetwork, it does not get an IPAddress from the podCIDR
		// but it uses the HostIP. Submariner, for such PODs, would MASQUERADE the sourceIP of
		// the outbound traffic (destined to remoteCluster) to the corresponding CNI interface
		// ip-address on that Host and globalIP will NOT be annotated on the POD.
		By("Verifying that globalIP annotation does not exist on the connector POD")
		podGlobalIP := connectorPod.Pod.GetAnnotations()[globalnetGlobalIPAnnotation]
		Expect(podGlobalIP).To(Equal(""))
	}

	// Return the pods in case further verification is needed
	return listenerPod, connectorPod
}

func RunNoConnectivityTest(p ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {
	if p.ConnectionTimeout == 0 {
		p.ConnectionTimeout = 5
	}

	if p.ConnectionAttempts == 0 {
		p.ConnectionAttempts = 1
	}

	listenerPod, connectorPod := createPods(&p)

	By("Verifying that listener pod exits with non-zero code and timed out message")
	Expect(listenerPod.TerminationMessage).To(ContainSubstring("nc: timeout"))
	Expect(listenerPod.TerminationCode).To(Equal(int32(1)))

	By("Verifying that connector pod exists with zero code but times out")
	Expect(connectorPod.TerminationMessage).To(ContainSubstring("Connection timed out"))
	Expect(connectorPod.TerminationCode).To(Equal(int32(0)))

	// Return the pods in case further verification is needed
	return listenerPod, connectorPod
}

func createPods(p *ConnectivityTestParams) (*framework.NetworkPod, *framework.NetworkPod) {

	By(fmt.Sprintf("Creating a listener pod in cluster %q, which will wait for a handshake over TCP", framework.TestContext.ClusterIDs[p.ToCluster]))
	listenerPod := p.Framework.NewNetworkPod(&framework.NetworkPodConfig{
		Type:               framework.ListenerPod,
		Cluster:            p.ToCluster,
		Scheduling:         p.ToClusterScheduling,
		ConnectionTimeout:  p.ConnectionTimeout,
		ConnectionAttempts: p.ConnectionAttempts,
	})

	remoteIP := listenerPod.Pod.Status.PodIP
	var service *v1.Service
	if p.ToEndpointType == ServiceIP || p.ToEndpointType == GlobalIP {
		By(fmt.Sprintf("Pointing a service ClusterIP to the listener pod in cluster %q", framework.TestContext.ClusterIDs[p.ToCluster]))
		service = listenerPod.CreateService()
		remoteIP = service.Spec.ClusterIP

		if p.ToEndpointType == GlobalIP {
			p.Framework.CreateServiceExport(p.ToCluster, service.Name)
			// Wait for the globalIP annotation on the service.
			service = p.Framework.AwaitUntilAnnotationOnService(p.ToCluster, globalnetGlobalIPAnnotation, service.Name, service.Namespace)
			remoteIP = service.GetAnnotations()[globalnetGlobalIPAnnotation]
		}
	}

	framework.Logf("Will send traffic to IP: %v", remoteIP)

	By(fmt.Sprintf("Creating a connector pod in cluster %q, which will attempt the specific UUID handshake over TCP", framework.TestContext.ClusterIDs[p.FromCluster]))
	connectorPod := p.Framework.NewNetworkPod(&framework.NetworkPodConfig{
		Type:               framework.ConnectorPod,
		Cluster:            p.FromCluster,
		Scheduling:         p.FromClusterScheduling,
		RemoteIP:           remoteIP,
		ConnectionTimeout:  p.ConnectionTimeout,
		ConnectionAttempts: p.ConnectionAttempts,
		Networking:         p.Networking,
	})

	if p.ToEndpointType == GlobalIP && p.Networking == framework.PodNetworking {
		// Wait for the globalIP annotation on the connectorPod.
		connectorPod.Pod = p.Framework.AwaitUntilAnnotationOnPod(p.FromCluster, globalnetGlobalIPAnnotation, connectorPod.Pod.Name, connectorPod.Pod.Namespace)
		sourceIP := connectorPod.Pod.GetAnnotations()[globalnetGlobalIPAnnotation]
		framework.Logf("Will send traffic from IP: %v", sourceIP)
	}

	By(fmt.Sprintf("Waiting for the connector pod %q on node %q to exit, returning what connector sent",
		connectorPod.Pod.Name, connectorPod.Pod.Spec.NodeName))
	connectorPod.AwaitFinish()

	By(fmt.Sprintf("Waiting for the listener pod %q on node %q to exit, returning what listener sent",
		listenerPod.Pod.Name, listenerPod.Pod.Spec.NodeName))
	listenerPod.AwaitFinish()

	framework.Logf("Connector pod has IP: %s", connectorPod.Pod.Status.PodIP)

	// In Globalnet deployments, when backend pods finish their execution, kubeproxy-iptables driver tries
	// to delete the iptables-chain associated with the service (even when the service is present) as there are
	// no active backend pods. Since the iptables-chain is also referenced by Globalnet Ingress rules, the chain
	// cannot be deleted (kubeproxy errors out and continues to retry) until Globalnet removes the reference.
	// Globalnet removes the reference only when the service itself is deleted. Until Globalnet is enhanced [*]
	// to remove this dependency with iptables-chain, lets delete the service after the listener Pod is terminated.
	// [*] https://github.com/submariner-io/submariner/issues/1166
	if p.ToEndpointType == GlobalIP {
		p.Framework.DeleteService(p.ToCluster, service.Name)
		p.Framework.DeleteServiceExport(p.ToCluster, service.Name)
	}

	return listenerPod, connectorPod
}
