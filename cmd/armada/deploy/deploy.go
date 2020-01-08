package deploy

import (
	"github.com/gobuffalo/packr/v2"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/armada/deploy/netshoot"
	"github.com/submariner-io/armada/cmd/armada/deploy/nginx"
)

// DeployCmd returns a new cobra.Command under root command for armada
func DeployCmd(box *packr.Box) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "deploy",
		Short: "Deploy resources",
		Long:  "Deploy resources",
	}
	cmd.AddCommand(netshoot.DeployNetshootCommand(box))
	cmd.AddCommand(nginx.DeployNginxDemoCommand(box))
	return cmd
}
