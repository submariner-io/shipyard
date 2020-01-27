package cluster

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
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

	// Log level to debug
	Debug bool

	// The number of clusters to create
	NumClusters int
}

// CreateClustersCommand returns a new cobra.Command under create command for armada
func NewCreateCommand(provider *kind.Provider, box *packr.Box) *cobra.Command {
	flags := &CreateFlagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "clusters",
		Short: "Creates multiple kubernetes clusters",
		Long:  "Creates multiple kubernetes clusters using Docker container 'nodes'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if flags.Debug {
				log.SetLevel(log.DebugLevel)
				//log.SetReportCaller(true)
			}

			targetClusters, err := GetTargetClusters(provider, flags)
			if err != nil {
				return err
			}

			tasks := []func() error{}
			for _, c := range targetClusters {
				config := c
				tasks = append(tasks, func() error {
					err := cluster.Create(config, provider, box)
					if err != nil {
						return fmt.Errorf("Error creating cluster %q", config.Name)
					}

					return nil
				})
			}

			err = wait.ForTasksComplete(defaults.WaitDurationResources, tasks...)
			if err != nil {
				return err
			}

			log.Info("Finalizing the clusters setup ...")

			tasks = []func() error{}
			for _, c := range targetClusters {
				config := c
				tasks = append(tasks, func() error {
					err := cluster.FinalizeSetup(config, box)
					if err != nil {
						return fmt.Errorf("Error finalizing cluster %q", config.Name)
					}

					return nil
				})
			}

			err = wait.ForTasksComplete(defaults.WaitDurationResources, tasks...)
			if err != nil {
				return err
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			files, err := ioutil.ReadDir(defaults.KindConfigDir)
			if err != nil {
				log.Fatal(err)
			}

			provider := kind.NewProvider()

			for _, file := range files {
				clName := strings.FieldsFunc(file.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
				known, err := cluster.IsKnown(clName, provider)
				if err != nil {
					log.Error(err)
				}
				if !known {
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
			}
			log.Infof("✔ Kubeconfigs: export KUBECONFIG=$(echo ./%s/kind-config-%s{1..%v} | sed 's/ /:/g')", defaults.LocalKubeConfigDir, defaults.ClusterNameBase, flags.NumClusters)
		},
	}
	cmd.Flags().StringVarP(&flags.ImageName, "image", "i", "", "node docker image to use for booting the cluster")
	cmd.Flags().BoolVarP(&flags.Retain, "retain", "", true, "retain nodes for debugging when cluster creation fails")
	cmd.Flags().StringVarP(&flags.Cni, "cni", "c", cluster.Kindnet, fmt.Sprintf("name of the cni that will be deployed on the cluster. Supported CNIs: %v", cluster.CNIs))
	cmd.Flags().BoolVarP(&flags.Tiller, "tiller", "t", false, "deploy with tiller")
	cmd.Flags().BoolVarP(&flags.Overlap, "overlap", "o", false, "create clusters with overlapping cidrs")
	cmd.Flags().BoolVarP(&flags.Debug, "debug", "v", false, "set log level to debug")
	cmd.Flags().DurationVar(&flags.Wait, "wait", 5*time.Minute, "amount of minutes to wait for control plane nodes to be ready")
	cmd.Flags().IntVarP(&flags.NumClusters, "num", "n", 2, "number of clusters to create")
	return cmd
}

// GetTargetClusters returns a list of clusters to create
func GetTargetClusters(provider *kind.Provider, flags *CreateFlagpole) ([]*cluster.Config, error) {
	var targetClusters []*cluster.Config
	for i := 1; i <= flags.NumClusters; i++ {
		clName := defaults.ClusterNameBase + strconv.Itoa(i)
		known, err := cluster.IsKnown(clName, provider)
		if err != nil {
			return nil, err
		}
		if known {
			log.Infof("✔ Cluster with the name %q already exists.", clName)
		} else {
			cl, err := cluster.PopulateConfig(i, flags.ImageName, flags.Cni, flags.Retain, flags.Tiller, flags.Overlap, flags.Wait)
			if err != nil {
				return nil, err
			}
			targetClusters = append(targetClusters, cl)
		}
	}
	return targetClusters, nil
}
