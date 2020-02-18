package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	dockerclient "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/image"
	"github.com/submariner-io/armada/pkg/utils"
	"github.com/submariner-io/armada/pkg/wait"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// loadFlagpole is a list of cli flags for export logs command
type loadFlagpole struct {
	clusters []string
	images   []string
}

// NewLoadCommand returns a new cobra.Command under load command for armada
func NewLoadCommand(provider *kind.Provider) *cobra.Command {
	flags := &loadFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "docker-images",
		Short: "Load docker images in to the cluster",
		Long:  "Load docker images in to the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			dockerCli, err := dockerclient.NewEnvClient()
			if err != nil {
				return err
			}

			clusters := utils.DetermineClusterNames(flags.clusters)
			if len(clusters) == 0 {
				return nil
			}

			for _, imageName := range flags.images {
				selectedNodes, err := image.GetNodesWithout(ctx, dockerCli, provider, imageName, clusters)
				if err != nil {
					log.Error(err)
				}
				if len(selectedNodes) == 0 {
					continue
				}

				imageTarPath, err := image.Save(ctx, dockerCli, imageName)
				if err != nil {
					return err
				}
				defer os.RemoveAll(filepath.Dir(imageTarPath))

				log.Infof("loading image: %s to nodes: %s ...", imageName, selectedNodes)

				tasks := []func() error{}
				for _, n := range selectedNodes {
					node := n
					tasks = append(tasks, func() error {
						err := image.LoadToNode(imageTarPath, imageName, node)
						if err != nil {
							return fmt.Errorf("Error loading image %q to node %q", imageName, node.String())
						}

						return nil
					})
				}

				err = wait.ForTasksComplete(defaults.WaitDurationResources, tasks...)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&flags.clusters, "clusters", "c", []string{}, "comma separated list of cluster names to load the image in to.")
	cmd.Flags().StringSliceVarP(&flags.images, "images", "i", []string{}, "comma separated list images to load.")
	return cmd
}
