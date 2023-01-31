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
)

const (
	fipsNamespace     = "kube-system"
	fipsConfigMapName = "cluster-config-v1"
)

func DetectFIPSConfig(cluster ClusterIndex) (bool, error) {
	configMap, err := KubeClients[cluster].CoreV1().ConfigMaps(fipsNamespace).Get(context.TODO(), fipsConfigMapName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return strings.Contains(configMap.Data["install-config"], "fips: true"), nil
}

func (f *Framework) FindFIPSEnabledCluster() ClusterIndex {
	for idx := range TestContext.ClusterIDs {
		fipsEnabled, err := DetectFIPSConfig(ClusterIndex(idx))
		Expect(err).NotTo(HaveOccurred())

		if fipsEnabled {
			return ClusterIndex(idx)
		}
	}

	return -1
}

func verifyFIPSOutput(data string) bool {
	return strings.Contains(strings.ToLower(data), "fips mode: yes") &&
		strings.Contains(strings.ToLower(data), "fips mode enabled for pluto daemon")
}

func (f *Framework) TestGatewayNodeFIPSMode(cluster ClusterIndex, gwPod string) {
	By(fmt.Sprintf("Verify FIPS mode is enabled on gateway pod %q", gwPod))

	ctx := context.TODO()
	cmd := []string{"ipsec", "pluto", "--selftest"}

	stdOut, stdErr, err := f.ExecWithOptions(ctx, &ExecOptions{
		Command:       cmd,
		Namespace:     TestContext.SubmarinerNamespace,
		PodName:       gwPod,
		ContainerName: SubmarinerGateway,
		CaptureStdout: true,
		CaptureStderr: true,
	}, cluster)
	Expect(err).To(Succeed())

	if stdOut == "" && stdErr == "" {
		Fail(fmt.Sprintf("No output received from command %q", cmd))
	}

	// The output of the "ipsec pluto --selftest" command could be written to stdout or stderr.
	// Checking both outputs for the expected strings.
	fipsStdOutResult := verifyFIPSOutput(stdOut)
	fipsStdErrResult := verifyFIPSOutput(stdErr)

	if fipsStdOutResult || fipsStdErrResult {
		By(fmt.Sprintf("FIPS mode is enabled on gateway pod %q", gwPod))
		return
	}

	Fail(fmt.Sprintf("FIPS mode is not enabled on gateway pod %q", gwPod))
}
