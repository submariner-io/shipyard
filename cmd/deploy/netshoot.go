package deploy

import (
	"sync"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/deploy"
	"github.com/submariner-io/armada/pkg/utils"
	"github.com/submariner-io/armada/pkg/wait"
)

// deployFlagpole is a list of cli flags for deploy nginx-demo command
type deployNetshootFlagpole struct {
	hostNetwork bool
	clusters    []string
}

func newDeployNetshootCommand(box *packr.Box) *cobra.Command {
	flags := &deployNetshootFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "netshoot",
		Short: "Deploy netshoot pods for debugging",
		Long:  "Deploy netshoot pods for debugging",
		RunE: func(cmd *cobra.Command, args []string) error {
			var netshootDeploymentFilePath string
			var selector string
			if flags.hostNetwork {
				netshootDeploymentFilePath = "debug/netshoot-daemonset-host.yaml"
				selector = "netshoot-host-net"
			} else {
				netshootDeploymentFilePath = "debug/netshoot-daemonset.yaml"
				selector = "netshoot"
			}

			netshootDeploymentFile, err := box.Resolve(netshootDeploymentFilePath)
			if err != nil {
				log.Error(err)
			}

			clusters := utils.DetermineClusterNames(flags.clusters)
			var wg sync.WaitGroup
			wg.Add(len(clusters))
			for _, clName := range clusters {
				go func(clName string) {
					client, err := cluster.NewClient(clName)
					if err != nil {
						log.Fatalf("%s %s", clName, err)
					}

					err = deploy.Resources(clName, client, netshootDeploymentFile.String(), "Netshoot")
					if err != nil {
						log.Fatalf("%s %s", clName, err)
					}

					err = wait.ForDaemonSetReady(clName, client, "default", selector)
					if err != nil {
						log.Fatalf("%s %s", clName, err)
					}
					wg.Done()
				}(clName)
			}
			wg.Wait()
			return nil
		},
	}
	cmd.Flags().BoolVar(&flags.hostNetwork, "host-network", false, "deploy the pods in host network mode.")
	cmd.Flags().StringSliceVarP(&flags.clusters, "clusters", "c", []string{}, "comma separated list of cluster names to deploy to. eg: cl1,cl6,cl3")
	return cmd
}
