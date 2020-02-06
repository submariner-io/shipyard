package cluster

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
	"github.com/submariner-io/armada/pkg/deploy"
	"github.com/submariner-io/armada/pkg/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Calico  = "calico"
	Flannel = "flannel"
	Kindnet = "kindnet"
	Weave   = "weave"
)

type deployCniFunc func(config *Config, box *packr.Box, client client.Client) error

var (
	CNIs = []string{Calico, Flannel, Kindnet, Weave}

	cniDeployers = map[string]deployCniFunc{Calico: deployCalico, Flannel: deployFlannel, Weave: deployWeave, Kindnet: deployKindnet}
)

func DeployCni(config *Config, box *packr.Box, client client.Client) error {
	deployCni, exists := cniDeployers[config.Cni]
	if !exists {
		return fmt.Errorf("Invalid CNI name %q", config.Cni)
	}

	return deployCni(config, box, client)
}

func deployFlannel(config *Config, box *packr.Box, client client.Client) error {
	flannelDeploymentFile, err := GenerateFlannelDeploymentFile(config, box)
	if err != nil {
		return err
	}

	err = deploy.Resources(config.Name, client, flannelDeploymentFile, "Flannel")
	if err != nil {
		return err
	}

	err = wait.ForDaemonSetReady(config.Name, client, "kube-system", "kube-flannel-ds-amd64")
	if err != nil {
		return err
	}

	return nil
}

func deployCalico(config *Config, box *packr.Box, client client.Client) error {
	calicoDeploymentFile, err := GenerateCalicoDeploymentFile(config, box)
	if err != nil {
		return err
	}

	calicoCrdFile, err := box.Resolve("tpl/calico-crd.yaml")
	if err != nil {
		return err
	}

	err = deploy.Resources(config.Name, client, calicoCrdFile.String(), "Calico CRDs")
	if err != nil {
		return err
	}

	err = deploy.Resources(config.Name, client, calicoDeploymentFile, "Calico")
	if err != nil {
		return err
	}

	err = wait.ForDaemonSetReady(config.Name, client, "kube-system", "calico-node")
	if err != nil {
		return err
	}

	err = wait.ForDeploymentReady(config.Name, client, "kube-system", "calico-kube-controllers")
	if err != nil {
		return err
	}

	return nil
}

func deployWeave(config *Config, box *packr.Box, client client.Client) error {
	weaveDeploymentFile, err := GenerateWeaveDeploymentFile(config, box)
	if err != nil {
		return err
	}

	err = deploy.Resources(config.Name, client, weaveDeploymentFile, "Weave")
	if err != nil {
		return err
	}

	err = wait.ForDaemonSetReady(config.Name, client, "kube-system", "weave-net")
	if err != nil {
		return err
	}

	return nil
}

func deployKindnet(config *Config, box *packr.Box, client client.Client) error {
	return nil
}

// GenerateCalicoDeploymentFile generates calico deployment file from template
func GenerateCalicoDeploymentFile(config *Config, box *packr.Box) (string, error) {
	return generateDeployment(config, box, "tpl/calico-daemonset.yaml")
}

// GenerateFlannelDeploymentFile generates flannel deployment file from template
func GenerateFlannelDeploymentFile(config *Config, box *packr.Box) (string, error) {
	return generateDeployment(config, box, "tpl/flannel-daemonset.yaml")
}

// GenerateWeaveDeploymentFile generates weave deployment file from template
func GenerateWeaveDeploymentFile(config *Config, box *packr.Box) (string, error) {
	return generateDeployment(config, box, "tpl/weave-daemonset.yaml")
}

func generateDeployment(config *Config, box *packr.Box, templateFileName string) (string, error) {
	templateFile, err := box.Resolve(templateFileName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find template file %q", templateFileName)
	}

	t, err := template.New(templateFileName).Parse(templateFile.String())
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse template file %q", templateFileName)
	}

	var deploymentBuffer bytes.Buffer
	err = t.Execute(&deploymentBuffer, config)
	if err != nil {
		return "", errors.Wrapf(err, "failed to generate deployment file from template %q", templateFileName)
	}

	return deploymentBuffer.String(), nil
}
