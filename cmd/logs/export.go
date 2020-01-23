package logs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/defaults"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// exportFlagpole is a list of cli flags for export logs command
type exportFlagpole struct {
	clusters []string
}

// NewExportCommand returns a new cobra.Command under export command for armada
func NewExportCommand(provider *kind.Provider) *cobra.Command {
	flags := &exportFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "logs",
		Short: "Export kind cluster logs",
		Long:  "Export kind cluster logs",
		RunE: func(cmd *cobra.Command, args []string) error {

			// remove existing before exporting
			_ = os.RemoveAll(filepath.Join(defaults.KindLogsDir, defaults.KindLogsDir))

			var targetClusters []string
			if len(flags.clusters) > 0 {
				targetClusters = append(targetClusters, flags.clusters...)
			} else {
				configFiles, err := ioutil.ReadDir(defaults.KindConfigDir)
				if err != nil {
					log.Fatal(err)
				}
				for _, configFile := range configFiles {
					clName := strings.FieldsFunc(configFile.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
					targetClusters = append(targetClusters, clName)
				}
			}
			for _, clName := range targetClusters {
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
