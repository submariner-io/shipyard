package cluster

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/utils"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// destroyFlagpole is a list of cli flags for destroy clusters command
type destroyFlagpole struct {
	clusters []string
}

// NewDestroyCommand returns a new cobra.Command under destroy command for armada
func NewDestroyCommand(provider *kind.Provider) *cobra.Command {
	flags := &destroyFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "clusters",
		Short: "Destroy clusters",
		Long:  "Destroys clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusters := utils.DetermineClusterNames(flags.clusters)
			for _, clName := range clusters {
				known, err := cluster.IsKnown(clName, provider)
				if err != nil {
					log.Fatalf("%s: %v", clName, err)
				}
				if known {
					err := cluster.Destroy(clName, provider)
					if err != nil {
						log.Fatalf("%s: %v", clName, err)
					}
				} else {
					log.Errorf("cluster %q not found.", clName)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&flags.clusters, "clusters", "c", []string{}, "comma separated list of cluster names to destroy. eg: cl1,cl6,cl3")
	return cmd
}
