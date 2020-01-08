package cluster_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
)

var _ = Describe("cni tests", func() {

	box := packr.New("configs", "../../configs")

	Context("Cni deployment files", func() {
		It("Should generate correct weave deployment file", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Name:      "cl1",
				PodSubnet: "1.2.3.4/14",
			}

			configDir := filepath.Join(currentDir, "testdata/cni")
			actual, err := cluster.GenerateWeaveDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())
			golden, err := ioutil.ReadFile(filepath.Join(configDir, "weave_deployment.golden"))
			Ω(err).ShouldNot(HaveOccurred())

			Expect(actual).Should(Equal(string(golden)))
		})
		It("Should generate correct flannel deployment file", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Name:      "cl1",
				PodSubnet: "1.2.3.4/8",
			}

			configDir := filepath.Join(currentDir, "testdata/cni")
			actual, err := cluster.GenerateFlannelDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())
			golden, err := ioutil.ReadFile(filepath.Join(configDir, "flannel_deployment.golden"))
			Ω(err).ShouldNot(HaveOccurred())

			Expect(actual).Should(Equal(string(golden)))
		})
		It("Should generate correct calico deployment file", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				PodSubnet: "1.2.3.4/16",
			}

			configDir := filepath.Join(currentDir, "testdata/cni")
			actual, err := cluster.GenerateCalicoDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())
			golden, err := ioutil.ReadFile(filepath.Join(configDir, "calico_deployment.golden"))
			Ω(err).ShouldNot(HaveOccurred())

			Expect(actual).Should(Equal(string(golden)))
		})
	})
})
