package cluster_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
)

var _ = Describe("Kubeconfig tests", func() {
	AfterSuite(func() {
		_ = os.RemoveAll("./output")
	})

	It("Should generate correct kube configs for local and container based deployments", func() {
		currentDir, err := os.Getwd()
		Expect(err).To(Succeed())

		clusterName := "cl1"

		configDir := filepath.Join(currentDir, "testdata/kube")
		kindKubeFileName := strings.Join([]string{"kind-config", clusterName}, "-")
		newLocalKubeFilePath := filepath.Join(currentDir, defaults.LocalKubeConfigDir, kindKubeFileName)
		newContainerKubeFilePath := filepath.Join(currentDir, defaults.ContainerKubeConfigDir, kindKubeFileName)

		gfs := filepath.Join(configDir, "kubeconfig_source")
		err = cluster.PrepareKubeConfigs(clusterName, gfs, "172.17.0.3")
		Expect(err).To(Succeed())

		local, err := ioutil.ReadFile(newLocalKubeFilePath)
		Expect(err).To(Succeed())

		container, err := ioutil.ReadFile(newContainerKubeFilePath)
		Expect(err).To(Succeed())

		localGolden, err := ioutil.ReadFile(filepath.Join(configDir, "kubeconfig_local.golden"))
		Expect(err).To(Succeed())

		containerGolden, err := ioutil.ReadFile(filepath.Join(configDir, "kubeconfig_container.golden"))
		Expect(err).To(Succeed())

		Expect(string(local)).Should(Equal(string(localGolden)))
		Expect(string(container)).Should(Equal(string(containerGolden)))
	})

	It("Should return correct kubeconfig file path", func() {
		got, err := cluster.GetKubeConfigPath("west")
		Expect(err).To(Succeed())
		Expect(got).To(HaveSuffix("kind-config-west"))
	})
})
