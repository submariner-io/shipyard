package load

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/image"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// NewCommand returns a new cobra.Command under root command for armada
func NewCommand(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "load",
		Short: "Load resources in to the cluster",
		Long:  "Load resources in to the cluster",
	}
	cmd.AddCommand(image.NewLoadCommand(provider))
	return cmd
}
