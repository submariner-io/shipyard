package cluster_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
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
