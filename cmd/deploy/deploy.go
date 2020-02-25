package deploy

import (
	"github.com/gobuffalo/packr/v2"
	"github.com/spf13/cobra"
)

// NewCommand returns a new cobra.Command under root command for armada
func NewCommand(box *packr.Box) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "deploy",
		Short: "Deploy one of [netshoot, nginx-demo]",
		Long:  "Deploy resources",
	}
	cmd.AddCommand(newDeployNetshootCommand(box))
	cmd.AddCommand(newDeployNginxCommand(box))
	return cmd
}
