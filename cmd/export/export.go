package export

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/logs"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// NewCommand returns a new cobra.Command under root command for armada
func NewCommand(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "export",
		Short: "Export data",
	}
	cmd.AddCommand(logs.NewExportCommand(provider))
	return cmd
}
