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

// Package dataplane runs the TCP/IP connectivity test.
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
