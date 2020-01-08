package export

import (
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/armada/export/logs"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// ExportCmd returns a new cobra.Command under root command for armada
func ExportCmd(provider *kind.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "export",
		Short: "Export kind cluster logs",
	}
	cmd.AddCommand(logs.ExportLogsCommand(provider))
	return cmd
}
