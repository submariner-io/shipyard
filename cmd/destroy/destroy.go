package destroy

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// NewCommand returns a new cobra.Command under root command for armada
func NewCommand(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "destroy",
		Short: "Destroys e2e environment",
		Long:  "Destroys multiple kind clusters",
	}
	cmd.AddCommand(cluster.NewDestroyCommand(provider))
	return cmd
}
