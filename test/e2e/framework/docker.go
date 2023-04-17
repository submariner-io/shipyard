/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

type Docker struct {
	Name string
}

func New(name string) *Docker {
	return &Docker{Name: name}
}

func (d *Docker) GetIP(networkName string) string {
	var stdout bytes.Buffer

	cmdargs := []string{
		"inspect", d.Name, "-f",
		fmt.Sprintf("{{(index .NetworkSettings.Networks %q).IPAddress}}", networkName),
	}
	cmd := exec.Command("docker", cmdargs...)
	cmd.Stdout = &stdout
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	// output has trailing "\n", so it needs to be trimed
	return strings.TrimSuffix(stdout.String(), "\n")
}

func (d *Docker) GetLog() (string, string) {
	var stdout, stderr bytes.Buffer

	// get stdout and stderr of `docker log {d.Name}` command
	// #nosec G204 -- the caller-controlled value is only used as the logs argument
	cmd := exec.Command("docker", "logs", d.Name)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	return stdout.String(), stderr.String()
}

func (d *Docker) runCommand(command ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer

	cmdargs := []string{"exec", "-i", d.Name}
	cmdargs = append(cmdargs, command...)
	cmd := exec.Command("docker", cmdargs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}

func (d *Docker) RunCommand(command ...string) (string, string) {
	stdout, stderr, err := d.runCommand(command...)
	Expect(err).NotTo(HaveOccurred())

	return stdout, stderr
}

func (d *Docker) RunCommandUntil(command ...string) (string, string) {
	var stdout, stderr string

	Eventually(func() error {
		var err error

		stdout, stderr, err = d.runCommand(command...)
		return err
	}, time.Duration(TestContext.OperationTimeout)*time.Second, 5*time.Second).Should(Succeed(),
		"Error attempting to run %v", append([]string{}, command...))

	return stdout, stderr
}
