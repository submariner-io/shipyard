package cluster

import (
	"bytes"
	"text/template"

	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
)

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
