package tcp

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/shipyard/test/e2e/framework"
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
	if p.ToEndpointType == ServiceIP || p.ToEndpointType == GlobalIP {
		By(fmt.Sprintf("Pointing a service ClusterIP to the listener pod in cluster %q", framework.TestContext.ClusterIDs[p.ToCluster]))
		service := listenerPod.CreateService()
		remoteIP = service.Spec.ClusterIP

		if p.ToEndpointType == GlobalIP {
			// Wait for the globalIP annotation on the service.
			service = p.Framework.AwaitServiceByAnnotation(p.ToCluster, globalnetGlobalIPAnnotation, service.Name, service.Namespace)
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

	By(fmt.Sprintf("Waiting for the listener pod %q to exit, returning what listener sent", listenerPod.Pod.Name))
	listenerPod.AwaitFinish()

	By(fmt.Sprintf("Waiting for the connector pod %q to exit, returning what connector sent", connectorPod.Pod.Name))
	connectorPod.AwaitFinish()

	framework.Logf("Connector pod has IP: %s", connectorPod.Pod.Status.PodIP)

	return listenerPod, connectorPod
}
