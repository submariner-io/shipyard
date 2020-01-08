package armada

import (
	"os"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/armada/create"
	"github.com/submariner-io/armada/cmd/armada/deploy"
	"github.com/submariner-io/armada/cmd/armada/destroy"
	"github.com/submariner-io/armada/cmd/armada/export"
	"github.com/submariner-io/armada/cmd/armada/load"
	"github.com/submariner-io/armada/cmd/armada/version"
	kind "sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

// Build and Version
var (
	Build   string
	Version string
)

// NewRootCmd returns a new cobra.Command implementing the root command for armada
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "armada",
		Short: "Armada is a tool for e2e environment creation for submariner-io org",
		Long:  "Creates multiple kind clusters and e2e environments",
	}

	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	provider := kind.NewProvider(
		kind.ProviderWithLogger(kindcmd.NewLogger()),
	)

	box := packr.New("configs", "../../configs")

	cmd.AddCommand(create.CreateCmd(provider, box))
	cmd.AddCommand(destroy.DestroyCmd(provider))
	cmd.AddCommand(export.ExportCmd(provider))
	cmd.AddCommand(load.LoadCmd(provider))
	cmd.AddCommand(deploy.DeployCmd(box))
	cmd.AddCommand(version.VersionCmd(Version, Build))
	return cmd
}

// Run runs the `armada` root command
func Run() error {
	return NewRootCmd().Execute()
}

// Main wraps Run
func Main() {
	// let's explicitly set stdout
	if err := Run(); err != nil {
		os.Exit(1)
	}
}
