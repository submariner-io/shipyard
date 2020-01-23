package cluster_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster test suite")
}

var _ = Describe("cluster tests", func() {

	box := packr.New("configs", "../../configs")

	Context("Kind config generation", func() {
		It("Should generate correct kind config for default cni", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Cni:                 "kindnet",
				Name:                "default",
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           "cl1.local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          2,
			}

			configDir := filepath.Join(currentDir, "testdata/kind")
			gf := filepath.Join(configDir, "default_cni.golden")
			configPath, err := cluster.GenerateKindConfig(cl, configDir, box)
			Ω(err).ShouldNot(HaveOccurred())

			golden, err := ioutil.ReadFile(gf)
			Ω(err).ShouldNot(HaveOccurred())
			actual, err := ioutil.ReadFile(configPath)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(actual)).Should(Equal(string(golden)))

			_ = os.RemoveAll(configPath)
		})

		It("Should generate correct kind config for custom cni", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Cni:                 "weave",
				Name:                "custom",
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           "cl1.local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          2,
			}

			configDir := filepath.Join(currentDir, "testdata/kind")
			gf := filepath.Join(configDir, "custom_cni.golden")
			configPath, err := cluster.GenerateKindConfig(cl, configDir, box)
			Ω(err).ShouldNot(HaveOccurred())

			golden, err := ioutil.ReadFile(gf)
			Ω(err).ShouldNot(HaveOccurred())
			actual, err := ioutil.ReadFile(configPath)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(actual)).Should(Equal(string(golden)))

			_ = os.RemoveAll(configPath)
		})

		It("Should generate correct kind config for cluster with 5 workers and custom cni", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			cl := &cluster.Config{
				Cni:                 "flannel",
				Name:                "custom",
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           "cl1.local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          5,
			}

			configDir := filepath.Join(currentDir, "testdata/kind")
			gf := filepath.Join(configDir, "custom_five_workers.golden")
			configPath, err := cluster.GenerateKindConfig(cl, configDir, box)
			Ω(err).ShouldNot(HaveOccurred())

			golden, err := ioutil.ReadFile(gf)
			Ω(err).ShouldNot(HaveOccurred())
			actual, err := ioutil.ReadFile(configPath)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(actual)).Should(Equal(string(golden)))

			_ = os.RemoveAll(configPath)
		})

		It("Should generate correct kind config for cluster with k8s version lower then 1.15", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			imageName := "test/test:v1.13.2"
			cni := "kindnet"
			cl, err := cluster.PopulateConfig(1, imageName, cni, true, true, false, 0)
			Ω(err).ShouldNot(HaveOccurred())

			configDir := filepath.Join(currentDir, "testdata/kind")
			gf := filepath.Join(configDir, "v1beta1.golden")
			cl.Name = "cl5"
			cl.DNSDomain = "cl5.local"
			configPath, err := cluster.GenerateKindConfig(cl, configDir, box)
			Ω(err).ShouldNot(HaveOccurred())

			golden, err := ioutil.ReadFile(gf)
			Ω(err).ShouldNot(HaveOccurred())
			actual, err := ioutil.ReadFile(configPath)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(actual)).Should(Equal(string(golden)))

			_ = os.RemoveAll(configPath)
		})

		It("Should generate correct kind config for cluster with k8s version higher then 1.15", func() {
			currentDir, err := os.Getwd()
			Ω(err).ShouldNot(HaveOccurred())

			imageName := "test/test:v1.16.2"
			cni := "kindnet"
			cl, err := cluster.PopulateConfig(1, imageName, cni, true, true, false, 0)
			Ω(err).ShouldNot(HaveOccurred())

			configDir := filepath.Join(currentDir, "testdata/kind")
			gf := filepath.Join(configDir, "v1beta2.golden")
			cl.Name = "cl8"
			cl.DNSDomain = "cl8.local"
			configPath, err := cluster.GenerateKindConfig(cl, configDir, box)
			Ω(err).ShouldNot(HaveOccurred())

			golden, err := ioutil.ReadFile(gf)
			Ω(err).ShouldNot(HaveOccurred())
			actual, err := ioutil.ReadFile(configPath)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(string(actual)).Should(Equal(string(golden)))

			_ = os.RemoveAll(configPath)
		})
	})

	Context("Containers", func() {
		ctx := context.Background()
		dockerCli, _ := dockerclient.NewEnvClient()

		BeforeEach(func() {
			reader, err := dockerCli.ImagePull(ctx, "docker.io/library/alpine:latest", types.ImagePullOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, reader)
			Ω(err).ShouldNot(HaveOccurred())

			resp, err := dockerCli.ContainerCreate(ctx, &container.Config{
				Image:  "alpine",
				Cmd:    []string{"/bin/sh"},
				Labels: map[string]string{"io.k8s.sigs.kind.cluster": "cl2"},
			}, nil, nil, "cl2-control-plane")
			Ω(err).ShouldNot(HaveOccurred())

			fmt.Print(resp)

			err = dockerCli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			containerFilter := filters.NewArgs()
			containerFilter.Add("name", "cl2-control-plane")

			containers, err := dockerCli.ContainerList(ctx, types.ContainerListOptions{
				Filters: containerFilter,
				Limit:   1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = dockerCli.ContainerRemove(ctx, containers[0].ID, types.ContainerRemoveOptions{
				Force: true,
			})
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("Should return the correct ip of a master node by name", func() {
			containerFilter := filters.NewArgs()
			containerFilter.Add("name", "cl2-control-plane")

			containers, err := dockerCli.ContainerList(ctx, types.ContainerListOptions{
				Filters: containerFilter,
				Limit:   1,
			})
			Ω(err).ShouldNot(HaveOccurred())
			Expect(len(containers)).ShouldNot(BeZero())

			actual := containers[0].NetworkSettings.Networks["bridge"].IPAddress
			masterIP, err := cluster.GetMasterDockerIP("cl2")
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Printf("actual: %s , returned: %s", actual, masterIP)

			Expect(actual).Should(Equal(masterIP))
		})

		It("Should return that the cluster is known", func() {

			provider := kind.NewProvider()

			got, err := cluster.IsKnown("cl2", provider)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(got).Should(BeTrue())
		})
	})
})
