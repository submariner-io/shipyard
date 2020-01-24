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

var _ = Describe("CNI tests", func() {
	box := packr.New("configs", "../../configs")

	Context("GenerateWeaveDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateWeaveDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/14"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "weave_deployment.golden")
		})
	})

	Context("GenerateFlannelDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateFlannelDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/8"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "flannel_deployment.golden")
		})
	})

	Context("GenerateCalicoDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateCalicoDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/16"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "calico_deployment.golden")
		})
	})
})

func verifyDeployment(actualDeploymentContents string, expDeploymentFileName string) {
	currentDir, err := os.Getwd()
	Expect(err).To(Succeed())
	configDir := filepath.Join(currentDir, "testdata/cni")

	expDeploymentFilePath := filepath.Join(configDir, expDeploymentFileName)
	expectedContents, err := ioutil.ReadFile(expDeploymentFilePath)
	Expect(err).To(Succeed())

	Expect(string(actualDeploymentContents)).Should(Equal(string(expectedContents)))
}
