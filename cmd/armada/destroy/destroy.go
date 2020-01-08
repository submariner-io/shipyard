package destroy

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/armada/destroy/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// DestroyCmd returns a new cobra.Command under root command for armada
func DestroyCmd(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "destroy",
		Short: "Destroys e2e environment",
		Long:  "Destroys multiple kind clusters",
	}
	cmd.AddCommand(cluster.DestroyClustersCommand(provider))
	return cmd
}
