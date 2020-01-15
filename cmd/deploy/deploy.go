package deploy

import (
	"github.com/gobuffalo/packr/v2"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/netshoot"
	"github.com/submariner-io/armada/cmd/nginx"
)

// NewCommand returns a new cobra.Command under root command for armada
func NewCommand(box *packr.Box) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "deploy",
		Short: "Deploy resources",
		Long:  "Deploy resources",
	}
	cmd.AddCommand(netshoot.NewDeployCommand(box))
	cmd.AddCommand(nginx.NewDeployCommand(box))
	return cmd
}
