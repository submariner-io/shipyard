package dataplane

import (
	. "github.com/onsi/ginkgo"
	"github.com/submariner-io/shipyard/test/e2e/framework"
	"github.com/submariner-io/shipyard/test/e2e/tcp"
)

var _ = Describe("[dataplane] Basic TCP connectivity test", func() {
	f := framework.NewFramework("dataplane")

	When("a pod connects to another pod via TCP", func() {
		It("should send the expected data to the other pod", func() {
			tcp.RunConnectivityTest(tcp.ConnectivityTestParams{
				Framework:             f,
				ToEndpointType:        tcp.PodIP,
				Networking:            framework.HostNetworking,
				FromCluster:           framework.ClusterA,
				FromClusterScheduling: framework.NonGatewayNode,
				ToCluster:             framework.ClusterA,
				ToClusterScheduling:   framework.NonGatewayNode,
			})
		})
	})
})
