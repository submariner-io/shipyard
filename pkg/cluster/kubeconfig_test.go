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

var _ = Describe("kubeconfig tests", func() {

	AfterSuite(func() {
		_ = os.RemoveAll("./output")
	})

	Context("Kubeconfigs", func() {
		It("Should generate correct kube configs for local and container based deployments", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Name: "cl1",
			}

			configDir := filepath.Join(currentDir, "testdata/kube")
			kindKubeFileName := strings.Join([]string{"kind-config", cl.Name}, "-")
			newLocalKubeFilePath := filepath.Join(currentDir, defaults.LocalKubeConfigDir, kindKubeFileName)
			newContainerKubeFilePath := filepath.Join(currentDir, defaults.ContainerKubeConfigDir, kindKubeFileName)
			gfs := filepath.Join(configDir, "kubeconfig_source")
			err = cluster.PrepareKubeConfigs(cl.Name, gfs, "172.17.0.3")
			Ω(err).ShouldNot(HaveOccurred())

			local, err := ioutil.ReadFile(newLocalKubeFilePath)
			Ω(err).ShouldNot(HaveOccurred())
			container, err := ioutil.ReadFile(newContainerKubeFilePath)
			Ω(err).ShouldNot(HaveOccurred())
			localGolden, err := ioutil.ReadFile(filepath.Join(configDir, "kubeconfig_local.golden"))
			Ω(err).ShouldNot(HaveOccurred())
			containerGolden, err := ioutil.ReadFile(filepath.Join(configDir, "kubeconfig_container.golden"))
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(local)).Should(Equal(string(localGolden)))
			Expect(string(container)).Should(Equal(string(containerGolden)))
		})
		It("Should return correct kubeconfig file path", func() {
			got, err := cluster.GetKubeConfigPath("cl1")
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).ShouldNot(BeNil())
		})
	})
})
