package cmd

import (
	"os"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/cmd/cluster"
	"github.com/submariner-io/armada/cmd/deploy"
	"github.com/submariner-io/armada/cmd/image"
	"github.com/submariner-io/armada/cmd/logs"
	"github.com/submariner-io/armada/cmd/version"
	kind "sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

// Build and Version
var (
	Build   string
	Version string
	Debug   bool
	cmd     = &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "armada",
		Short: "Armada is a tool for e2e environment creation for submariner-io org",
		Long:  "Creates multiple kind clusters and e2e environments",
	}
)

func setLogDebug() {
	if Debug {
		log.SetLevel(log.DebugLevel)
	}
}

func init() {
	cobra.OnInitialize(setLogDebug)
	cmd.PersistentFlags().BoolVarP(&Debug, "debug", "v", false, "set log level to debug")
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	provider := kind.NewProvider(
		kind.ProviderWithLogger(kindcmd.NewLogger()),
	)

	box := packr.New("configs", "../configs")

	cmd.AddCommand(cluster.NewCreateCommand(provider, box))
	cmd.AddCommand(cluster.NewDestroyCommand(provider))
	cmd.AddCommand(logs.NewExportCommand(provider))
	cmd.AddCommand(image.NewLoadCommand(provider))
	cmd.AddCommand(deploy.NewCommand(box))
	cmd.AddCommand(version.NewCommand(Version, Build))
}

// Run runs the `armada` root command
func Run() error {
	return cmd.Execute()
}

// Main wraps Run
func Main() {
	// let's explicitly set stdout
	if err := Run(); err != nil {
		os.Exit(1)
	}
}
