package image_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/image"
)

func TestImage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image test suite")
}

var _ = Describe("image tests", func() {
	Context("images", func() {
		ctx := context.Background()
		dockerCli, _ := dockerclient.NewEnvClient()

		BeforeSuite(func() {
			reader, err := dockerCli.ImagePull(ctx, "docker.io/library/alpine:latest", types.ImagePullOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, reader)
			Ω(err).ShouldNot(HaveOccurred())
		})
		It("Should return the correct local imageID", func() {
			log.SetLevel(log.DebugLevel)
			imageFilter := filters.NewArgs()
			imageFilter.Add("reference", "alpine:latest")
			result, err := dockerCli.ImageList(ctx, types.ImageListOptions{
				All:     false,
				Filters: imageFilter,
			})
			Ω(err).ShouldNot(HaveOccurred())
			imageID, err := image.GetLocalID(ctx, dockerCli, "alpine:latest")
			Ω(err).ShouldNot(HaveOccurred())

			Expect(result[0].ID).Should(Equal(imageID))
		})
		It("Should save the image to temp location", func() {
			log.SetLevel(log.DebugLevel)
			tempFilePath, err := image.Save(ctx, dockerCli, "alpine:latest")
			Ω(err).ShouldNot(HaveOccurred())
			defer os.RemoveAll(filepath.Dir(tempFilePath))

			file, err := os.Stat(tempFilePath)
			Ω(err).ShouldNot(HaveOccurred())
			size := file.Size()
			log.Infof("temp file size: %v", size)
			Expect(size).ShouldNot(BeZero())
		})
	})
})
