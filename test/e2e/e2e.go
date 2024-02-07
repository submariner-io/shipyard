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

package e2e

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/submariner-io/shipyard/test/e2e/framework"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// There are certain operations we only want to run once per overall test invocation
// (such as deleting old namespaces, or verifying that all system pods are running.
// Because of the way Ginkgo runs tests in parallel, we must use SynchronizedBeforeSuite
// to ensure that these operations only run on the first parallel Ginkgo node.
//
// This function takes two parameters: one function which runs on only the first Ginkgo node,
// returning an opaque byte array, and then a second function which runs on all Ginkgo nodes,
// accepting the byte array.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Run only on Ginkgo node 1

	framework.BeforeSuite()
	return nil
}, func(_ []byte) {
	// Run on all Ginkgo nodes
})

// Similar to SynchornizedBeforeSuite, we want to run some operations only once (such as collecting cluster logs).
// Here, the order of functions is reversed; first, the function which runs everywhere,
// and then the function that only runs on the first Ginkgo node.

var _ = SynchronizedAfterSuite(func() {
	// Run on all Ginkgo nodes

	// framework.Logf("Running AfterSuite actions on all node")
	framework.RunCleanupActions()
}, func() {
	// Run only Ginkgo on node 1
})

func init() {
	klog.InitFlags(nil)
}

func RunE2ETests(t *testing.T) bool {
	framework.SetStatusFunction(func(text string) {
		By(text)
	})

	framework.SetFailFunction(func(text string) {
		Fail(text)
	})

	framework.SetUserAgentFunction(func() string {
		return fmt.Sprintf("%v -- %v", rest.DefaultKubernetesUserAgent(), CurrentSpecReport().FullText())
	})

	framework.ValidateFlags(framework.TestContext)
	gomega.RegisterFailHandler(Fail)

	suiteConfig, reporterConfig := GinkgoConfiguration()

	if framework.TestContext.SuiteConfig != nil {
		suiteConfig = *framework.TestContext.SuiteConfig
	}

	if framework.TestContext.ReporterConfig != nil {
		reporterConfig = *framework.TestContext.ReporterConfig
	}

	return RunSpecs(t, "Submariner E2E suite", suiteConfig, reporterConfig)
}
