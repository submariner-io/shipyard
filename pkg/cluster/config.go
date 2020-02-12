package cluster

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver"

	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/utils"
)

// Config type
type Config struct {
	// The name of the cni that will be installed for a cluster
	Cni string

	// The cluster name
	Name string

	// The pod subnet cidr and mask
	PodSubnet string

	// The a service subnet cidr and mask
	ServiceSubnet string

	// The cluster dns domain name
	DNSDomain string

	// The KubeAdminAPIVersion for the cluster
	KubeAdminAPIVersion string

	// The number of worker nodes
	NumWorkers int

	// The destination where kind will generate the original kubeconfig file
	KubeConfigFilePath string

	// The amount of time to wait for control plain to be ready
	WaitForReady time.Duration

	// The config image name
	NodeImageName string

	// Whether or not to keep clusters running even if error occurs
	Retain bool

	// Whether or not to install tiller
	Tiller bool
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

// GenerateKindConfig creates kind config file and returns its path
func GenerateKindConfig(config *Config, configDir string, box *packr.Box) (kindConfigFilePath string, err error) {
	templateFile := "tpl/cluster-config.yaml"
	kindConfigTemplateFile, err := box.Resolve(templateFile)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find kind config template file %q", templateFile)
	}

	kindConfigTemplate, err := template.New("config").Funcs(template.FuncMap{"iterate": iterate}).Parse(kindConfigTemplateFile.String())
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse kind config template file %q", templateFile)
	}

	kindConfigFilePath = filepath.Join(configDir, "kind-config-"+config.Name+".yaml")
	kindConfigFile, err := os.Create(kindConfigFilePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create kind config file %q", kindConfigFilePath)
	}

	defer func() {
		if err == nil {
			err = kindConfigFile.Close()
		}

		if err != nil {
			os.Remove(kindConfigFilePath)
			kindConfigFilePath = ""
		}
	}()

	err = kindConfigTemplate.Execute(kindConfigFile, config)
	if err != nil {
		err = errors.Wrapf(err, "failed to generated kind config file %q", kindConfigFilePath)
		return
	}

	log.Debugf("Generated kind config config file %q", kindConfigFilePath)
	return
}

// PopulateConfig return a desired cluster config object
func PopulateConfig(clusterNum int, image, cni string, retain, tiller, overlap bool, wait time.Duration) (*Config, error) {
	user, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current user information")
	}

	name := utils.ClusterName(clusterNum)
	config := &Config{
		Name:                name,
		NodeImageName:       image,
		Cni:                 cni,
		NumWorkers:          defaults.NumWorkers,
		DNSDomain:           name + ".local",
		KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
		Retain:              retain,
		Tiller:              tiller,
		WaitForReady:        wait,
		KubeConfigFilePath:  filepath.Join(user.HomeDir, ".kube", "kind-config-"+name),
	}

	podIP := net.ParseIP(defaults.PodCidrBase)
	podIP = podIP.To4()
	serviceIP := net.ParseIP(defaults.ServiceCidrBase)
	serviceIP = serviceIP.To4()

	if !overlap {
		podIP[1] += byte(4 * clusterNum)
		serviceIP[1] += byte(clusterNum)
	}

	config.PodSubnet = podIP.String() + defaults.PodCidrMask
	config.ServiceSubnet = serviceIP.String() + defaults.ServiceCidrMask

	if cni != "kindnet" {
		config.WaitForReady = 0
	}

	if image != "" {
		tgt := semver.MustParse("1.15")
		results := strings.Split(image, ":v")
		if len(results) == 2 {
			sver := semver.MustParse(results[len(results)-1])
			if sver.LessThan(tgt) {
				config.KubeAdminAPIVersion = "kubeadm.k8s.io/v1beta1"
			}
		} else {
			return nil, errors.Errorf("%q: Could not extract version from image %q. Split is by ':v' - example of correct image name: kindest/node:v1.15.3.", config.Name, config.NodeImageName)
		}
	}
	return config, nil
}
