package create

import (
	"github.com/gobuffalo/packr/v2"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// NewCommand returns a new cobra.Command under the root command for armada
func NewCommand(provider *kind.Provider, box *packr.Box) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "create",
		Short: "Creates e2e environment",
		Long:  "Creates multiple kind based clusters",
	}

	cmd.AddCommand(cluster.NewCreateCommand(provider, box))
	return cmd
}
