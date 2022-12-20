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
	"flag"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

type contextArray []string

type TestContextType struct {
	KubeConfigs         []string // KubeConfigs provides an alternative to KubeConfig + KubeContexts
	KubeConfig          string
	KubeContexts        contextArray
	ClusterIDs          []string
	NumNodesInCluster   map[ClusterIndex]int
	JunitReport         string
	SubmarinerNamespace string
	ConnectionTimeout   uint
	ConnectionAttempts  uint
	OperationTimeout    uint
	GlobalnetEnabled    bool
	ClientQPS           float32
	ClientBurst         int
	GroupVersion        *schema.GroupVersion
	NettestImageURL     string
}

func (contexts *contextArray) String() string {
	return strings.Join(*contexts, ",")
}

func (contexts *contextArray) Set(value string) error {
	*contexts = append(*contexts, value)
	return nil
}

var TestContext = &TestContextType{
	ClientQPS:       20,
	ClientBurst:     50,
	NettestImageURL: "quay.io/submariner/nettest:devel",
}

func init() {
	flag.StringVar(&TestContext.KubeConfig, "kubeconfig", os.Getenv("KUBECONFIG"),
		"Path to kubeconfig containing embedded authinfo.")
	flag.Var(&TestContext.KubeContexts, "dp-context", "kubeconfig context for dataplane clusters (use several times).")
	flag.StringVar(&TestContext.JunitReport, "junit-report", "",
		"Path to the directory and filename of the JUnit XML report. Default is empty, which doesn't generate these reports.")
	flag.StringVar(&TestContext.SubmarinerNamespace, "submariner-namespace", "submariner",
		"Namespace in which the submariner components are deployed.")
	flag.UintVar(&TestContext.ConnectionTimeout, "connection-timeout", 18,
		"The timeout in seconds per connection attempt when verifying communication between clusters.")
	flag.UintVar(&TestContext.ConnectionAttempts, "connection-attempts", 7,
		"The number of connection attempts when verifying communication between clusters.")
	flag.UintVar(&TestContext.OperationTimeout, "operation-timeout", 190, "The general operation timeout in seconds.")
}

func ValidateFlags(t *TestContextType) {
	if t.KubeConfig == "" && len(t.KubeConfigs) == 0 {
		klog.Fatalf("kubeconfig parameter or KUBECONFIG environment variable is required")
	}

	if len(t.KubeContexts) < 1 && len(t.KubeConfigs) < 1 {
		klog.Fatalf("at least one kubernetes context must be specified.")
	}
}
