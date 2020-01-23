package cluster

import (
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
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
