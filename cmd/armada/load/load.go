package load

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/armada/load/image"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// LoadCmd returns a new cobra.Command under root command for armada
func LoadCmd(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "load",
		Short: "Load resources in to hte cluster",
		Long:  "Load resources in to hte cluster",
	}
	cmd.AddCommand(image.LoadImageCommand(provider))
	return cmd
}
