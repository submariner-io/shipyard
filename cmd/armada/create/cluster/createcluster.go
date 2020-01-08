package cluster

import (
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// CreateClusterFlagpole is a list of cli flags for create clusters command
type CreateClusterFlagpole struct {
	// ImageName is the node image used for cluster creation
	ImageName string

	// Wait is a time duration to wait until cluster is ready
	Wait time.Duration

	// Retain if you keep clusters running even if error occurs
	Retain bool

	// Weave if to install weave cni
	Weave bool

	// Flannel if to install flannel cni
	Flannel bool

	// Calico if to install calico cni
	Calico bool

	// Kindnet if to install kindnet default cni
	Kindnet bool

	// DeployTiller if to install tiller
	Tiller bool

	// Overlap if to create clusters with overlapping cidrs
	Overlap bool

	// Debug sets log level to debug
	Debug bool

	// NumClusters is the number of clusters to create
	NumClusters int
}

// CreateClustersCommand returns a new cobra.Command under create command for armada
func CreateClustersCommand(provider *kind.Provider, box *packr.Box) *cobra.Command {
	flags := &CreateClusterFlagpole{}
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
				log.Fatal(err)
			}

			var wg sync.WaitGroup
			wg.Add(len(targetClusters))
			for _, cl := range targetClusters {
				go func(cl *cluster.Config) {
					err := cluster.Create(cl, provider, box, &wg)
					if err != nil {
						defer wg.Done()
						log.Fatalf("%s: %s", cl.Name, err)
					}
				}(cl)
			}
			wg.Wait()

			log.Info("Finalizing the clusters setup ...")
			wg.Add(len(targetClusters))
			for _, cl := range targetClusters {
				go func(cl *cluster.Config) {
					err := cluster.FinalizeSetup(cl, box, &wg)
					if err != nil {
						defer wg.Done()
						log.Fatalf("%s: %s", cl.Name, err)
					}
				}(cl)
			}
			wg.Wait()
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
	cmd.Flags().BoolVarP(&flags.Weave, "weave", "w", false, "deploy with weave")
	cmd.Flags().BoolVarP(&flags.Tiller, "tiller", "t", false, "deploy with tiller")
	cmd.Flags().BoolVarP(&flags.Calico, "calico", "c", false, "deploy with calico")
	cmd.Flags().BoolVarP(&flags.Kindnet, "kindnet", "k", true, "deploy with kindnet default cni")
	cmd.Flags().BoolVarP(&flags.Flannel, "flannel", "f", false, "deploy with flannel")
	cmd.Flags().BoolVarP(&flags.Overlap, "overlap", "o", false, "create clusters with overlapping cidrs")
	cmd.Flags().BoolVarP(&flags.Debug, "debug", "v", false, "set log level to debug")
	cmd.Flags().DurationVar(&flags.Wait, "wait", 5*time.Minute, "amount of minutes to wait for control plane nodes to be ready")
	cmd.Flags().IntVarP(&flags.NumClusters, "num", "n", 2, "number of clusters to create")
	return cmd
}

// GetTargetClusters returns a list of clusters to create
func GetTargetClusters(provider *kind.Provider, flags *CreateClusterFlagpole) ([]*cluster.Config, error) {
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
			cni := GetCniFromFlags(flags)
			cl, err := cluster.PopulateConfig(i, flags.ImageName, cni, flags.Retain, flags.Tiller, flags.Overlap, flags.Wait)
			if err != nil {
				return nil, err
			}
			targetClusters = append(targetClusters, cl)
		}
	}
	return targetClusters, nil
}

// GetCniFromFlags returns the cni name from flags
func GetCniFromFlags(flags *CreateClusterFlagpole) string {
	var cni string
	if flags.Weave {
		cni = "weave"
	} else if flags.Flannel {
		cni = "flannel"
	} else if flags.Calico {
		cni = "calico"
	} else if flags.Kindnet {
		cni = "kindnet"
	}
	return cni
}
