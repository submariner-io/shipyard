package logs

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/utils"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// exportFlagpole is a list of cli flags for export logs command
type exportFlagpole struct {
	clusters []string
}

// NewExportCommand returns a new cobra.Command capable of exporting logs
func NewExportCommand(provider *kind.Provider) *cobra.Command {
	flags := &exportFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "export-logs",
		Short: "Export kind cluster logs",
		Long:  "Export kind cluster logs",
		RunE: func(cmd *cobra.Command, args []string) error {

			// remove existing before exporting
			_ = os.RemoveAll(filepath.Join(defaults.KindLogsDir, defaults.KindLogsDir))

			clusters := utils.DetermineClusterNames(flags.clusters)
			for _, clName := range clusters {
				err := provider.CollectLogs(clName, filepath.Join(defaults.KindLogsDir, clName))
				if err != nil {
					log.Fatalf("%s: %v", clName, err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&flags.clusters, "clusters", "c", []string{}, "comma separated list of cluster names. eg: cluster1,cluster6,cluster3")
	return cmd
}
