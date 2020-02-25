package deploy

import (
	"sync"

	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/deploy"
	"github.com/submariner-io/armada/pkg/utils"
	"github.com/submariner-io/armada/pkg/wait"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NginxDeployFlagpole is a list of cli flags for deploy nginx-demo command
type deployNginxFlagpole struct {
	clusters []string
}

func newDeployNginxCommand(box *packr.Box) *cobra.Command {
	flags := &deployNginxFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "nginx-demo",
		Short: "Deploy nginx demo application service and pods",
		Long:  "Deploy nginx demo application service and pods",
		RunE: func(cmd *cobra.Command, args []string) error {
			nginxDeploymentFile, err := box.Resolve("debug/nginx-demo-daemonset.yaml")
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

					err = deploy.Resources(clName, client, nginxDeploymentFile.String(), "Nginx")
					if err != nil {
						log.Fatalf("%s %s", clName, err)
					}

					err = wait.ForDaemonSetReady(clName, client, "default", "nginx-demo")
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
	cmd.Flags().StringSliceVarP(&flags.clusters, "clusters", "c", []string{}, "comma separated list of cluster names to deploy to. eg: cl1,cl6,cl3")
	return cmd
}
