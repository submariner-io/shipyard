package cluster

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/utils"
	"github.com/submariner-io/armada/pkg/wait"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// CreateFlagpole is a list of cli flags for create clusters command
type CreateFlagpole struct {
	// The node image used for cluster creation
	ImageName string

	// Time duration to wait until cluster is ready
	Wait time.Duration

	// Whether or not to keep clusters running even if error occurs
	Retain bool

	// The name of the cni that will be installed for the cluster
	Cni string

	// Whether or not to install tiller
	Tiller bool

	// Whether or not to create clusters with overlapping cidrs
	Overlap bool

	// The number of clusters to create
	NumClusters int
}

// NewCreateCommand returns a new cobra.Command that can create multiple clusters
func NewCreateCommand(provider *kind.Provider, box *packr.Box) *cobra.Command {
	flags := &CreateFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "create",
		Short: "Creates multiple kubernetes clusters",
		Long:  "Creates multiple kubernetes clusters using Docker container 'nodes'",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := CreateClusters(flags, provider, box)
			return err
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			persistClusterKubeconfigs(flags)
		},
	}
	cmd.Flags().StringVarP(&flags.ImageName, "image", "i", "", "node docker image to use for booting the cluster")
	cmd.Flags().BoolVarP(&flags.Retain, "retain", "", true, "retain nodes for debugging when cluster creation fails")
	cmd.Flags().StringVarP(&flags.Cni, "cni", "c", cluster.Kindnet, fmt.Sprintf("name of the cni that will be deployed on the cluster. Supported CNIs: %v", cluster.CNIs))
	cmd.Flags().BoolVarP(&flags.Tiller, "tiller", "t", false, "deploy with tiller")
	cmd.Flags().BoolVarP(&flags.Overlap, "overlap", "o", false, "create clusters with overlapping cidrs")
	cmd.Flags().DurationVar(&flags.Wait, "wait", 5*time.Minute, "amount of minutes to wait for control plane nodes to be ready")
	cmd.Flags().IntVarP(&flags.NumClusters, "num", "n", 2, "number of clusters to create")
	return cmd
}

// CreateClusters will create the requested clusters while waiting for their creation to finish
func CreateClusters(flags *CreateFlagpole, provider *kind.Provider, box *packr.Box) ([]*cluster.Config, error) {
	targetClusters, err := getTargetClusters(provider, flags)
	if err != nil {
		return nil, err
	}

	tasks := []func() error{}
	for _, c := range targetClusters {
		config := c
		tasks = append(tasks, func() error {
			err := cluster.Create(config, provider, box)
			return errors.Wrapf(err, "Error creating cluster %q", config.Name)
		})
	}

	err = wait.ForTasksComplete(defaults.WaitDurationResources, tasks...)
	if err != nil {
		return nil, err
	}

	log.Info("Finalizing the clusters setup ...")

	tasks = []func() error{}
	for _, c := range targetClusters {
		config := c
		tasks = append(tasks, func() error {
			err := cluster.FinalizeSetup(config, box)
			return errors.Wrapf(err, "Error finalizing cluster %q", config.Name)
		})
	}

	err = wait.ForTasksComplete(defaults.WaitDurationResources, tasks...)
	if err != nil {
		return nil, err
	}

	return targetClusters, nil
}

func persistClusterKubeconfigs(flags *CreateFlagpole) {
	clusters, err := utils.ClusterNamesFromFiles()
	if err != nil {
		log.Fatal(err)
	}

	provider := kind.NewProvider()

	for _, clName := range clusters {
		known, err := cluster.IsKnown(clName, provider)
		if err != nil {
			log.Error(err)
		}
		if known {
			continue
		}

		usr, err := user.Current()
		if err != nil {
			log.Error(err)
		}

		kindKubeFileName := strings.Join([]string{"kind-config", clName}, "-")
		kindKubeFilePath := filepath.Join(usr.HomeDir, ".kube", kindKubeFileName)

		masterIP, err := cluster.GetMasterDockerIP(clName)
		if err != nil {
			log.Error(err)
		}

		err = cluster.PrepareKubeConfigs(clName, kindKubeFilePath, masterIP)
		if err != nil {
			log.Error(err)
		}
	}
	log.Infof("✔ Kubeconfigs: export KUBECONFIG=$(echo ./%s/kind-config-%s{1..%v} | sed 's/ /:/g')", defaults.LocalKubeConfigDir, defaults.ClusterNameBase, flags.NumClusters)
}

// getTargetClusters returns a list of clusters to create
func getTargetClusters(provider *kind.Provider, flags *CreateFlagpole) ([]*cluster.Config, error) {
	var targetClusters []*cluster.Config
	for i := 1; i <= flags.NumClusters; i++ {
		clName := utils.ClusterName(i)
		known, err := cluster.IsKnown(clName, provider)
		if err != nil {
			return nil, err
		}
		if known {
			log.Infof("✔ Cluster with the name %q already exists.", clName)
			continue
		}

		cl, err := cluster.PopulateConfig(i, flags.ImageName, flags.Cni, flags.Retain, flags.Tiller, flags.Overlap, flags.Wait)
		if err != nil {
			return nil, err
		}
		targetClusters = append(targetClusters, cl)
	}
	return targetClusters, nil
}
