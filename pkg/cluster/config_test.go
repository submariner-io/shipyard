package cluster_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/utils"
)

const waitForReady = 5 * time.Minute

var _ = Describe("Kind config tests", func() {
	Context("PopulateConfig function", testPopulateConfig)
	Context("GenerateKindConfig function", testGenerateKindConfig)
})

func testGenerateKindConfig() {
	box := packr.New("configs", "../../configs")

	var (
		config     *cluster.Config
		configDir  string
		configPath string
	)

	BeforeEach(func() {
		config = &cluster.Config{
			Cni:                 "kindnet",
			Name:                "default",
			PodSubnet:           "10.4.0.0/14",
			ServiceSubnet:       "100.1.0.0/16",
			DNSDomain:           "cl1.local",
			KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
			NumWorkers:          2,
		}
	})

	JustBeforeEach(func() {
		currentDir, err := os.Getwd()
		Expect(err).To(Succeed())

		configDir = filepath.Join(currentDir, "testdata/kind")
		configPath, err = cluster.GenerateKindConfig(config, configDir, box)
		Expect(err).To(Succeed())
	})

	JustAfterEach(func() {
		_ = os.RemoveAll(configPath)
	})

	When("the CNI is kindnet", func() {
		It("should generate the correct kind config file", func() {
			verifyKindConfigFile(configDir, configPath, "default_cni.golden")
		})
	})

	When("the CNI is custom", func() {
		BeforeEach(func() {
			config.Name = "custom"
			config.Cni = "weave"
		})

		It("should generate the correct kind config file", func() {
			verifyKindConfigFile(configDir, configPath, "custom_cni.golden")
		})
	})

	When("the number of workers is 5", func() {
		BeforeEach(func() {
			config.NumWorkers = 5
		})

		It("should generate the correct kind config file", func() {
			verifyKindConfigFile(configDir, configPath, "custom_five_workers.golden")
		})
	})
}

func verifyKindConfigFile(configDir string, actualConfigFilePath string, expectedConfigFileName string) {
	expectedConfigFilePath := filepath.Join(configDir, expectedConfigFileName)

	expected, err := ioutil.ReadFile(expectedConfigFilePath)
	Expect(err).To(Succeed())

	actual, err := ioutil.ReadFile(actualConfigFilePath)
	Expect(err).To(Succeed())

	Expect(string(actual)).Should(Equal(string(expected)))
}

func testPopulateConfig() {
	When("the CNI is kindnet and the kind image name is empty", func() {
		It("should correctly set the Config fields", func() {
			config := executePopulateConfig(1, "", "kindnet", true)

			user, err := user.Current()
			Expect(err).To(Succeed())

			name := utils.ClusterName(1)
			Expect(config).Should(Equal(&cluster.Config{
				Cni:                 "kindnet",
				Name:                name,
				PodSubnet:           "10.0.0.0/14",
				ServiceSubnet:       "100.0.0.0/16",
				DNSDomain:           name + ".local",
				KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(user.HomeDir, ".kube", "kind-config-"+name),
				WaitForReady:        waitForReady,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
	})

	When("the kind image version is less than 1.15", func() {
		It("should set the KubeAdminAPIVersion to kubeadm.k8s.io/v1beta1", func() {
			imageName := "kindest/node:v1.11.1"
			config := executePopulateConfig(1, imageName, "kindnet", true)
			Expect(config.KubeAdminAPIVersion).To(Equal("kubeadm.k8s.io/v1beta1"))
			Expect(config.NodeImageName).To(Equal(imageName))
		})
	})

	When("the kind image version is greater than 1.15", func() {
		It(fmt.Sprintf("should set the KubeAdminAPIVersion to %s", defaults.KubeAdminAPIVersion), func() {
			imageName := "kindest/node:v1.16.3"
			config := executePopulateConfig(1, imageName, "kindnet", true)
			Expect(config.KubeAdminAPIVersion).To(Equal(defaults.KubeAdminAPIVersion))
			Expect(config.NodeImageName).To(Equal(imageName))
		})
	})

	When("the kind image version is invalid", func() {
		It("should return an error", func() {
			_, err := cluster.PopulateConfig(1, "kindest/node:1.16.3", "kindnet", false, false, true, waitForReady)
			Expect(err).ToNot(Succeed())
		})
	})

	When("the CNI is weave", func() {
		It("should set WaitForReady to 0", func() {
			config := executePopulateConfig(1, "", "weave", true)
			Expect(config.Cni).To(Equal("weave"))
			Expect(config.WaitForReady).To(Equal(time.Duration(0)))
		})
	})

	When("the CNI is calico", func() {
		It("should set WaitForReady to 0", func() {
			config := executePopulateConfig(1, "", "calico", true)
			Expect(config.Cni).To(Equal("calico"))
			Expect(config.WaitForReady).To(Equal(time.Duration(0)))
		})
	})

	When("the CNI is flannel", func() {
		It("should set WaitForReady to 0", func() {
			config := executePopulateConfig(1, "", "flannel", true)
			Expect(config.Cni).To(Equal("flannel"))
			Expect(config.WaitForReady).To(Equal(time.Duration(0)))
		})
	})

	When("non-overlapping IPs are desired", func() {
		It("should correctly set the pod and service subnets CIDRs to be different", func() {
			expectedPodSubnets := []string{"10.4.0.0/14", "10.8.0.0/14"}
			expectedServiceSubnets := []string{"100.1.0.0/16", "100.2.0.0/16"}
			for i := 0; i < 2; i++ {
				config := executePopulateConfig(i+1, "", "kindnet", false)
				Expect(config.PodSubnet).To(Equal(expectedPodSubnets[i]))
				Expect(config.ServiceSubnet).To(Equal(expectedServiceSubnets[i]))
				Expect(config.Name).To(Equal(utils.ClusterName(i + 1)))
			}
		})
	})
}

func executePopulateConfig(clusterNum int, imageName, cni string, overlap bool) *cluster.Config {
	config, err := cluster.PopulateConfig(clusterNum, imageName, cni, false, false, overlap, waitForReady)
	Expect(err).To(Succeed())
	return config
}
