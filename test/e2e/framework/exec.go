package framework

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Command []string

	Namespace     string
	PodName       string
	ContainerName string

	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
}

// ExecWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func (f *Framework) ExecWithOptions(options ExecOptions, index ClusterIndex) (string, string, error) {
	Logf("ExecWithOptions %+v", options)

	config, _, err := loadConfig(TestContext.KubeConfig, TestContext.KubeContexts[index])
	Expect(err).To(Succeed(), fmt.Sprintf("ExecWithOptions %#v", options))

	const tty = false
	req := f.ClusterClients[index].CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)

	req.VersionedParams(&v1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       tty,
	}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	attempts := 5
	for ; attempts > 0; attempts-- {
		err = execute("POST", req.URL(), config, options.Stdin, &stdout, &stderr, tty)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 5000)
		Logf("Retrying due to error  %+v", err)
	}

	if options.PreserveWhitespace {
		return stdout.String(), stderr.String(), err
	}

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}
