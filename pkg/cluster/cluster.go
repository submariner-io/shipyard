package cluster

import (
	"context"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/deploy"
	"github.com/submariner-io/armada/pkg/wait"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kind "sigs.k8s.io/kind/pkg/cluster"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// Create creates cluster with kind
func Create(cl *Config, provider *kind.Provider, box *packr.Box, wg *sync.WaitGroup) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	configDir := filepath.Join(currentDir, defaults.KindConfigDir)
	err = os.MkdirAll(configDir, os.ModePerm)
	if err != nil {
		return err
	}

	kindConfigFilePath, err := GenerateKindConfig(cl, configDir, box)
	if err != nil {
		return err
	}

	raw, err := ioutil.ReadFile(kindConfigFilePath)
	if err != nil {
		return err
	}

	log.Infof("Creating cluster %q, cni: %s, podcidr: %s, servicecidr: %s, workers: %v.", cl.Name, cl.Cni, cl.PodSubnet, cl.ServiceSubnet, cl.NumWorkers)

	if err = provider.Create(
		cl.Name,
		kind.CreateWithRawConfig(raw),
		kind.CreateWithNodeImage(cl.NodeImageName),
		kind.CreateWithKubeconfigPath(cl.KubeConfigFilePath),
		kind.CreateWithRetain(cl.Retain),
		kind.CreateWithWaitForReady(cl.WaitForReady),
		kind.CreateWithDisplayUsage(false),
		kind.CreateWithDisplaySalutation(false),
	); err != nil {
		if errs := kinderrors.Errors(err); errs != nil {
			for _, problem := range errs {
				return problem
			}
			return errors.New("aborting due to invalid configuration")
		}
		return errors.Wrap(err, "failed to create cluster")
	}
	wg.Done()
	return nil
}

// Destroy destroys a kind cluster
func Destroy(clName string, provider *kind.Provider) error {
	log.Infof("Deleting cluster %q ...\n", clName)
	if err := provider.Delete(clName, ""); err != nil {
		return errors.Wrapf(err, "failed to delete cluster %s", clName)
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(defaults.KindConfigDir, "kind-config-"+clName+".yaml"))
	_ = os.Remove(filepath.Join(defaults.LocalKubeConfigDir, "kind-config-"+clName))
	_ = os.Remove(filepath.Join(defaults.ContainerKubeConfigDir, "kind-config-"+clName))
	_ = os.RemoveAll(filepath.Join(usr.HomeDir, ".kube", strings.Join([]string{"kind-config", clName}, "-")))
	_ = os.RemoveAll(filepath.Join(defaults.KindLogsDir, clName))

	return nil
}

// GetMasterDockerIP gets control plain master docker internal ip
func GetMasterDockerIP(clName string) (string, error) {
	ctx := context.Background()
	dockerCli, err := dockerclient.NewEnvClient()
	if err != nil {
		return "", err
	}

	containerFilter := filters.NewArgs()
	containerFilter.Add("name", strings.Join([]string{clName, "control-plane"}, "-"))
	containers, err := dockerCli.ContainerList(ctx, dockertypes.ContainerListOptions{
		Filters: containerFilter,
		Limit:   1,
	})
	if err != nil {
		return "", err
	}
	return containers[0].NetworkSettings.Networks["bridge"].IPAddress, nil
}

// iterate func map for config template
func iterate(start, end int) (stream chan int) {
	stream = make(chan int)
	go func() {
		for i := start; i <= end; i++ {
			stream <- i
		}
		close(stream)
	}()
	return
}

// IsKnown returns bool if cluster exists
func IsKnown(clName string, provider *kind.Provider) (bool, error) {
	n, err := provider.ListNodes(clName)
	if err != nil {
		return false, err
	}
	if len(n) != 0 {
		return true, nil
	}
	return false, nil
}

// NewClient creates a new client.Client instance for the given cluster name.
func NewClient(cluster string) (client.Client, error) {
	kubeConfigFilePath, err := GetKubeConfigPath(cluster)
	if err != nil {
		return nil, err
	}

	kconfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	if err != nil {
		return nil, err
	}

	return client.New(kconfig, client.Options{})
}

// FinalizeSetup creates custom environment
func FinalizeSetup(cl *Config, box *packr.Box, wg *sync.WaitGroup) error {
	masterIP, err := GetMasterDockerIP(cl.Name)
	if err != nil {
		return err
	}

	err = PrepareKubeConfigs(cl.Name, cl.KubeConfigFilePath, masterIP)
	if err != nil {
		return err
	}

	client, err := NewClient(cl.Name)
	if err != nil {
		return err
	}

	switch cl.Cni {
	case "calico":
		calicoDeploymentFile, err := GenerateCalicoDeploymentFile(cl, box)
		if err != nil {
			return err
		}

		calicoCrdFile, err := box.Resolve("tpl/calico-crd.yaml")
		if err != nil {
			return err
		}

		err = deploy.Resources(cl.Name, client, calicoCrdFile.String(), "Calico CRD")
		if err != nil {
			return err
		}

		err = deploy.Resources(cl.Name, client, calicoDeploymentFile, "Calico")
		if err != nil {
			return err
		}

		err = wait.ForDaemonSetReady(cl.Name, client, "kube-system", "calico-node")
		if err != nil {
			return err
		}

		err = wait.ForDeploymentReady(cl.Name, client, "kube-system", "coredns")
		if err != nil {
			return err
		}
	case "flannel":
		flannelDeploymentFile, err := GenerateFlannelDeploymentFile(cl, box)
		if err != nil {
			return err
		}

		err = deploy.Resources(cl.Name, client, flannelDeploymentFile, "Flannel")
		if err != nil {
			return err
		}

		err = wait.ForDaemonSetReady(cl.Name, client, "kube-system", "kube-flannel-ds-amd64")
		if err != nil {
			return err
		}

		err = wait.ForDeploymentReady(cl.Name, client, "kube-system", "coredns")
		if err != nil {
			return err
		}
	case "weave":
		weaveDeploymentFile, err := GenerateWeaveDeploymentFile(cl, box)
		if err != nil {
			return err
		}

		err = deploy.Resources(cl.Name, client, weaveDeploymentFile, "Weave")
		if err != nil {
			return err
		}

		err = wait.ForDaemonSetReady(cl.Name, client, "kube-system", "weave-net")
		if err != nil {
			return err
		}

		err = wait.ForDeploymentReady(cl.Name, client, "kube-system", "coredns")
		if err != nil {
			return err
		}
	}

	if cl.Tiller {
		tillerDeploymentFile, err := box.Resolve("helm/tiller-deployment.yaml")
		if err != nil {
			return err
		}

		err = deploy.Resources(cl.Name, client, tillerDeploymentFile.String(), "Tiller")
		if err != nil {
			return err
		}

		err = wait.ForDeploymentReady(cl.Name, client, "kube-system", "tiller-deploy")
		if err != nil {
			return err
		}
	}
	log.Infof("✔ Cluster %q is ready 🔥🔥🔥", cl.Name)
	wg.Done()
	return nil
}
