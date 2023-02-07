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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type NetworkingType bool

const (
	HostNetworking NetworkingType = true
	PodNetworking  NetworkingType = false
	dataPrefixSize uint           = 25 // size of prefix string '[dataplane] listener says'
)

type NetworkPodType int

const (
	InvalidPodType NetworkPodType = iota
	ListenerPod
	ConnectorPod
	ThroughputClientPod
	ThroughputServerPod
	LatencyClientPod
	LatencyServerPod
	CustomPod
)

type NetworkPodScheduling int

const (
	InvalidScheduling NetworkPodScheduling = iota
	GatewayNode
	NonGatewayNode
)

type NetworkPodConfig struct {
	Type               NetworkPodType
	Cluster            ClusterIndex
	Scheduling         NetworkPodScheduling
	Port               int
	Data               string
	NumOfDataBufs      uint
	RemoteIP           string
	ConnectionTimeout  uint
	ConnectionAttempts uint
	Networking         NetworkingType
	ContainerName      string
	ImageName          string
	Command            []string
	// TODO: namespace, once https://github.com/submariner-io/submariner/pull/141 is merged
}

type NetworkPod struct {
	Pod                 *v1.Pod
	Config              *NetworkPodConfig
	TerminationError    error
	TerminationErrorMsg string
	TerminationCode     int32
	TerminationMessage  string
	framework           *Framework
}

const (
	TestPort = 1234
)

func (f *Framework) NewNetworkPod(config *NetworkPodConfig) *NetworkPod {
	// check if all necessary details are provided
	Expect(config.Scheduling).ShouldNot(Equal(InvalidScheduling))
	Expect(config.Type).ShouldNot(Equal(InvalidPodType))

	// setup unset defaults
	if config.Port == 0 {
		config.Port = TestPort
	}

	if config.Data == "" {
		config.Data = string(uuid.NewUUID())
	}

	if TestContext.PacketSize == 0 {
		config.NumOfDataBufs = 50
	} else {
		config.NumOfDataBufs = 1 + (TestContext.PacketSize / (uint(len(config.Data)) + dataPrefixSize))
	}

	networkPod := &NetworkPod{Config: config, framework: f, TerminationCode: -1}

	switch config.Type {
	case ListenerPod:
		networkPod.buildTCPCheckListenerPod()
	case ConnectorPod:
		networkPod.buildTCPCheckConnectorPod()
	case ThroughputClientPod:
		networkPod.buildThroughputClientPod()
	case ThroughputServerPod:
		networkPod.buildThroughputServerPod()
	case LatencyClientPod:
		networkPod.buildLatencyClientPod()
	case LatencyServerPod:
		networkPod.buildLatencyServerPod()
	case CustomPod:
		networkPod.buildCustomPod()
	case InvalidPodType:
		panic("config.Type can't equal InvalidPodType here, we checked above")
	}

	return networkPod
}

func (np *NetworkPod) AwaitReady() {
	pods := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)

	np.Pod = AwaitUntil("await pod ready", func() (interface{}, error) {
		return pods.Get(context.TODO(), np.Pod.Name, metav1.GetOptions{})
	}, func(result interface{}) (bool, string, error) {
		pod := result.(*v1.Pod)
		if pod.Status.Phase != v1.PodRunning {
			if pod.Status.Phase != v1.PodPending {
				return false, "", fmt.Errorf("unexpected pod phase %v - expected %v or %v", pod.Status.Phase, v1.PodPending, v1.PodRunning)
			}

			out, _ := json.MarshalIndent(&pod.Status, "", "  ")

			return false, fmt.Sprintf("Pod %q is still pending: status:\n%s", pod.Name, out), nil
		}

		return true, "", nil // pod is running
	}).(*v1.Pod)
}

func (np *NetworkPod) AwaitFinish() {
	np.AwaitFinishVerbose(true)
}

func (np *NetworkPod) AwaitFinishVerbose(verbose bool) {
	pods := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)

	_, np.TerminationErrorMsg, np.TerminationError = AwaitResultOrError(fmt.Sprintf("await pod %q finished", np.Pod.Name),
		func() (interface{}, error) {
			return pods.Get(context.TODO(), np.Pod.Name, metav1.GetOptions{})
		}, func(result interface{}) (bool, string, error) {
			np.Pod = result.(*v1.Pod)

			if np.Pod.Status.Phase == v1.PodSucceeded || np.Pod.Status.Phase == v1.PodFailed {
				return true, "", nil
			}

			return false, fmt.Sprintf("Pod status is %v", np.Pod.Status.Phase), nil
		})

	finished := np.Pod.Status.Phase == v1.PodSucceeded || np.Pod.Status.Phase == v1.PodFailed
	if finished {
		np.TerminationCode = np.Pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
		np.TerminationMessage = np.Pod.Status.ContainerStatuses[0].State.Terminated.Message

		if verbose {
			Logf("Pod %q on node %q output:\n%s", np.Pod.Name, np.Pod.Spec.NodeName, removeDupDataplaneLines(np.TerminationMessage))
		}
	}
}

func (np *NetworkPod) CheckSuccessfulFinish() {
	Expect(np.TerminationError).NotTo(HaveOccurred(), np.TerminationErrorMsg)
	Expect(np.TerminationCode).To(Equal(int32(0)))
}

func (np *NetworkPod) CreateService() *v1.Service {
	return np.framework.CreateTCPService(np.Config.Cluster, np.Pod.Labels[TestAppLabel], np.Config.Port)
}

// RunCommand run the specified command in this NetworkPod.
func (np *NetworkPod) RunCommand(ctx context.Context, cmd []string) (string, string) {
	req := KubeClients[np.Config.Cluster].CoreV1().RESTClient().Post().
		Resource("pods").Name(np.Pod.Name).Namespace(np.Pod.Namespace).
		SubResource("exec").Param("container", np.Config.ContainerName)

	req.VersionedParams(&v1.PodExecOptions{
		Container: np.Config.ContainerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	config := RestConfigs[np.Config.Cluster]

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	Expect(err).NotTo(HaveOccurred())

	var stdout, stderr bytes.Buffer

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	Expect(err).NotTo(HaveOccurred())

	return stdout.String(), stderr.String()
}

// GetLog returns container log from this NetworkPod.
func (np *NetworkPod) GetLog() string {
	req := KubeClients[np.Config.Cluster].CoreV1().Pods(np.Pod.Namespace).GetLogs(np.Pod.Name, &v1.PodLogOptions{})

	closer, err := req.Stream(context.TODO())
	Expect(err).NotTo(HaveOccurred())

	defer closer.Close()

	out := new(strings.Builder)

	_, err = io.Copy(out, closer)
	Expect(err).NotTo(HaveOccurred())

	return out.String()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will listen on TestPort over TCP, send sendString over the connection,
// and write the network response in the pod  termination log, then exit with 0 status.
func (np *NetworkPod) buildTCPCheckListenerPod() {
	tcpCheckListenerPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "tcp-check-listener",
			Labels: map[string]string{
				TestAppLabel: "tcp-check-listener",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:  "tcp-check-listener",
					Image: TestContext.NettestImageURL,
					// We send the string 50 times to put more pressure on the TCP connection and avoid limited
					// resource environments from not sending at least some data before timeout.
					Command: []string{
						"sh",
						"-c",
						"for i in $(seq 1 $BUFS_NUM);" +
							" do echo [dataplane] listener says $SEND_STRING;" +
							" done" +
							" | nc -w $CONN_TIMEOUT -l -v -p $LISTEN_PORT -s 0.0.0.0 >/dev/termination-log 2>&1",
					},
					Env: []v1.EnvVar{
						{Name: "LISTEN_PORT", Value: strconv.Itoa(np.Config.Port)},
						{Name: "SEND_STRING", Value: np.Config.Data},
						{Name: "CONN_TIMEOUT", Value: strconv.Itoa(int(np.Config.ConnectionTimeout * np.Config.ConnectionAttempts))},
						{Name: "BUFS_NUM", Value: strconv.Itoa(int(np.Config.NumOfDataBufs))},
					},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}

	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &tcpCheckListenerPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	np.AwaitReady()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will connect to remoteIP:TestPort over TCP, send sendString over the
// connection, and write the network response in the pod termination log, then
// exit with 0 status.
func (np *NetworkPod) buildTCPCheckConnectorPod() {
	tcpCheckConnectorPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "tcp-check-pod",
			Labels: map[string]string{
				TestAppLabel: "tcp-check-pod",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			HostNetwork:   bool(np.Config.Networking),
			Containers: []v1.Container{
				{
					Name:  "tcp-check-connector",
					Image: TestContext.NettestImageURL,
					// We send the string 50 times to put more pressure on the TCP connection and avoid limited
					// resource environments from not sending at least some data before timeout.
					Command: []string{
						"sh",
						"-c",
						"for in in $(seq 1 $BUFS_NUM);" +
							" do echo [dataplane] connector says $SEND_STRING; done" +
							" | for i in $(seq $CONN_TRIES);" +
							" do if nc -v $REMOTE_IP $REMOTE_PORT -w $CONN_TIMEOUT;" +
							" then break;" +
							" else sleep $RETRY_SLEEP;" +
							" fi; done >/dev/termination-log 2>&1",
					},
					Env: []v1.EnvVar{
						{Name: "REMOTE_PORT", Value: strconv.Itoa(np.Config.Port)},
						{Name: "SEND_STRING", Value: np.Config.Data},
						{Name: "REMOTE_IP", Value: np.Config.RemoteIP},
						{Name: "CONN_TRIES", Value: strconv.Itoa(int(np.Config.ConnectionAttempts))},
						{Name: "CONN_TIMEOUT", Value: strconv.Itoa(int(np.Config.ConnectionTimeout))},
						{Name: "RETRY_SLEEP", Value: strconv.Itoa(int(np.Config.ConnectionTimeout / 2))},
						{Name: "BUFS_NUM", Value: strconv.Itoa(int(np.Config.NumOfDataBufs))},
					},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}

	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &tcpCheckConnectorPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will initiate iperf3 throughput test to remoteIP and write the test
// response in the pod termination log, then
// exit with 0 status.
func (np *NetworkPod) buildThroughputClientPod() {
	nettestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "nettest-client-pod",
			Labels: map[string]string{
				TestAppLabel: "nettest-client-pod",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:            "nettest-client-pod",
					Image:           TestContext.NettestImageURL,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"sh", "-c", "for i in $(seq $CONN_TRIES);" +
							" do if iperf3 -w 256K --connect-timeout $CONN_TIMEOUT -P 10 -p $TARGET_PORT -c $TARGET_IP;" +
							" then break;" +
							" else echo [going to retry]; sleep $RETRY_SLEEP;" +
							" fi; done >/dev/termination-log 2>&1",
					},
					Env: []v1.EnvVar{
						{Name: "TARGET_IP", Value: np.Config.RemoteIP},
						{Name: "TARGET_PORT", Value: strconv.Itoa(np.Config.Port)},
						{Name: "CONN_TRIES", Value: strconv.Itoa(int(np.Config.ConnectionAttempts))},
						{Name: "RETRY_SLEEP", Value: strconv.Itoa(int(np.Config.ConnectionTimeout))},
						{Name: "CONN_TIMEOUT", Value: strconv.Itoa(int(np.Config.ConnectionTimeout * 1000))},
					},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}
	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &nettestPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	np.AwaitReady()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will start iperf3 in server mode.
func (np *NetworkPod) buildThroughputServerPod() {
	nettestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "nettest-server-pod",
			Labels: map[string]string{
				TestAppLabel: "nettest-server-pod",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:            "nettest-server-pod",
					Image:           TestContext.NettestImageURL,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"sh", "-c", "iperf3 -s -p $TARGET_PORT"},
					Env: []v1.EnvVar{
						{Name: "TARGET_PORT", Value: strconv.Itoa(np.Config.Port)},
					},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}
	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &nettestPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	np.AwaitReady()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will initiate netperf latency test to remoteIP and write the test
// response in the pod termination log, then
// exit with 0 status.
func (np *NetworkPod) buildLatencyClientPod() {
	nettestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "latency-client-pod",
			Labels: map[string]string{
				TestAppLabel: "latency-client-pod",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:            "latency-client-pod",
					Image:           TestContext.NettestImageURL,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"sh",
						"-c",
						"netperf -H $TARGET_IP -t TCP_RR  -- -o min_latency,mean_latency,max_latency,stddev_latency,transaction_rate" +
							" >/dev/termination-log 2>&1",
					},
					Env: []v1.EnvVar{
						{Name: "TARGET_IP", Value: np.Config.RemoteIP},
					},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}
	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &nettestPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	np.AwaitReady()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will start netserver (server of netperf).
func (np *NetworkPod) buildLatencyServerPod() {
	nettestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "latency-server-pod",
			Labels: map[string]string{
				TestAppLabel: "latency-server-pod",
			},
		},
		Spec: v1.PodSpec{
			Affinity:      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:            "latency-server-pod",
					Image:           TestContext.NettestImageURL,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"netserver", "-D"},
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}
	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)
	var err error
	np.Pod, err = pc.Create(context.TODO(), &nettestPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	np.AwaitReady()
}

// create a test pod inside the current test namespace on the specified cluster.
// The pod will use the image specified and run command specified.
func (np *NetworkPod) buildCustomPod() {
	terminationGracePeriodSeconds := int64(5)
	customPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "custom",
			Labels: map[string]string{
				TestAppLabel: "custom",
			},
		},
		Spec: v1.PodSpec{
			Affinity:                      np.nodeAffinity(np.Config.Scheduling),
			RestartPolicy:                 v1.RestartPolicyNever,
			HostNetwork:                   bool(np.Config.Networking),
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []v1.Container{
				{
					Name:            np.Config.ContainerName,
					Image:           np.Config.ImageName,
					ImagePullPolicy: v1.PullAlways,
					Command:         np.Config.Command,
					SecurityContext: podSecurityContext,
				},
			},
			Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
		},
	}

	pc := KubeClients[np.Config.Cluster].CoreV1().Pods(np.framework.Namespace)

	var err error
	np.Pod, err = pc.Create(context.TODO(), &customPod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	np.AwaitReady()
}

func (np *NetworkPod) nodeAffinity(scheduling NetworkPodScheduling) *v1.Affinity {
	Expect(scheduling).ShouldNot(Equal(InvalidScheduling))

	var nodeSelTerms []v1.NodeSelectorTerm

	switch scheduling {
	case GatewayNode:
		smGWPodList, err := KubeClients[np.Config.Cluster].CoreV1().Pods(TestContext.SubmarinerNamespace).List(context.TODO(),
			metav1.ListOptions{LabelSelector: ActiveGatewayLabel})

		Expect(err).NotTo(HaveOccurred())
		Expect(len(smGWPodList.Items)).To(Equal(1))
		hostname := smGWPodList.Items[0].GetObjectMeta().GetLabels()["gateway.submariner.io/node"]
		Expect(len(hostname)).NotTo(BeZero())
		nodeSelTerms = addNodeSelectorTerm(nodeSelTerms, "kubernetes.io/hostname", v1.NodeSelectorOpIn, []string{hostname})

	case NonGatewayNode:
		nodeSelTerms = addNodeSelectorTerm(nodeSelTerms, GatewayLabel, v1.NodeSelectorOpDoesNotExist, nil)
		nodeSelTerms = addNodeSelectorTerm(nodeSelTerms, GatewayLabel, v1.NodeSelectorOpNotIn, []string{"true"})

	case InvalidScheduling:
		panic("scheduling can't equal InvalidScheduling here, we checked above")
	}

	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: nodeSelTerms,
			},
		},
	}
}

func addNodeSelectorTerm(nodeSelTerms []v1.NodeSelectorTerm, label string,
	op v1.NodeSelectorOperator, values []string,
) []v1.NodeSelectorTerm {
	return append(nodeSelTerms, v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
		{
			Key:      label,
			Operator: op,
			Values:   values,
		},
	}})
}

func removeDupDataplaneLines(output string) string {
	var newLines []string
	var lastLine string

	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "[dataplane]") || line != lastLine {
			newLines = append(newLines, line)
		}

		lastLine = line
	}

	return strings.Join(newLines, "\n")
}
