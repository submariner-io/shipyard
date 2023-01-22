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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	mcsv1a1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

const (
	// Polling interval while trying to create objects.
	PollInterval = 100 * time.Millisecond
)

type ClusterIndex int

const (
	ClusterA ClusterIndex = iota
	ClusterB
	ClusterC
)

const (
	SubmarinerGateway  = "submariner-gateway"
	RouteAgent         = "submariner-routeagent"
	GatewayLabel       = "submariner.io/gateway"
	ActiveGatewayLabel = "gateway.submariner.io/status=active"
)

type PatchFunc func(pt types.PatchType, payload []byte) error

type PatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type PatchUInt32Value struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint32 `json:"value"`
}

type (
	DoOperationFunc func() (interface{}, error)
	CheckResultFunc func(result interface{}) (bool, string, error)
)

// Framework supports common operations used by e2e tests; it will keep a client & a namespace for you.
// Eventual goal is to merge this with integration test framework.
type Framework struct {
	BaseName string

	// Set together with creating the ClientSet and the namespace.
	// Guaranteed to be unique in the cluster even when running the same
	// test multiple times in parallel.
	UniqueName               string
	SkipNamespaceCreation    bool            // Whether to skip creating a namespace
	Namespace                string          // Every test has a namespace at least unless creation is skipped
	namespacesToDelete       map[string]bool // Some tests have more than one.
	NamespaceDeletionTimeout time.Duration
	gatewayNodesToReset      map[int][]string // Store GW nodes for the final cleanup

	// To make sure that this framework cleans up after itself, no matter what,
	// we install a Cleanup action before each test and clear it after.  If we
	// should abort, the AfterSuite hook should run all Cleanup actions.
	cleanupHandle CleanupActionHandle
}

var (
	beforeSuiteFuncs []func()

	RestConfigs []*rest.Config
	KubeClients []*kubeclientset.Clientset
	DynClients  []dynamic.Interface

	podSecurityContext *corev1.SecurityContext
)

// NewBareFramework creates a test framework, without ginkgo dependencies.
func NewBareFramework(baseName string) *Framework {
	return &Framework{
		BaseName:            baseName,
		namespacesToDelete:  map[string]bool{},
		gatewayNodesToReset: map[int][]string{},
	}
}

func AddBeforeSuite(beforeSuite func()) {
	beforeSuiteFuncs = append(beforeSuiteFuncs, beforeSuite)
}

var (
	By                func(string)
	Fail              func(string)
	userAgentFunction func() string
)

func SetStatusFunction(by func(string)) {
	By = by
}

func SetFailFunction(fail func(string)) {
	Fail = fail
}

func SetUserAgentFunction(uaf func() string) {
	userAgentFunction = uaf
}

func init() {
	By = func(str string) {
		fmt.Println(str)
	}
	Fail = func(str string) {
		panic("Framework Fail:" + str)
	}
	userAgentFunction = func() string {
		return "shipyard-framework-agent"
	}
}

func BeforeSuite() {
	By("Creating kubernetes clients")
	Expect(1).To(BeZero())

	if len(RestConfigs) == 0 {
		if len(TestContext.KubeConfig) > 0 {
			Expect(len(TestContext.KubeConfigs)).To(BeZero(),
				"Either KubeConfig or KubeConfigs must be specified but not both")

			for _, ctx := range TestContext.KubeContexts {
				RestConfigs = append(RestConfigs, createRestConfig(TestContext.KubeConfig, ctx))
			}

			// if cluster IDs are not provided we assume that cluster-id == context
			if len(TestContext.ClusterIDs) == 0 {
				TestContext.ClusterIDs = TestContext.KubeContexts
			}
		} else if len(TestContext.KubeConfigs) > 0 {
			Expect(len(TestContext.KubeConfigs)).To(Equal(len(TestContext.ClusterIDs)),
				"One ClusterID must be provided for each item in the KubeConfigs")
			for _, kubeConfig := range TestContext.KubeConfigs {
				RestConfigs = append(RestConfigs, createRestConfig(kubeConfig, ""))
			}
		} else {
			Fail("One of KubeConfig or KubeConfigs must be specified")
		}
	}

	KubeClients = nil
	DynClients = nil

	for _, restConfig := range RestConfigs {
		KubeClients = append(KubeClients, createKubernetesClient(restConfig))
		DynClients = append(DynClients, createDynamicClient(restConfig))
	}

	fetchClusterIDs()

	err := mcsv1a1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	for _, beforeSuite := range beforeSuiteFuncs {
		beforeSuite()
	}

	initPodSecurityContext()
}

func initPodSecurityContext() {
	podSecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		RunAsNonRoot: pointer.Bool(true),
		RunAsUser:    pointer.Int64(10000), // We need to set some user ID other than 0.
	}

	serverVersion, err := KubeClients[0].Discovery().ServerVersion()
	Expect(err).To(Succeed())

	major, err := strconv.Atoi(serverVersion.Major)
	Expect(err).To(Succeed())

	var minor int
	if strings.HasSuffix(serverVersion.Minor, "+") {
		minor, err = strconv.Atoi(serverVersion.Minor[0 : len(serverVersion.Minor)-1])
	} else {
		minor, err = strconv.Atoi(serverVersion.Minor)
	}

	Expect(err).To(Succeed())

	if major > 1 || minor >= 24 {
		podSecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	}
}

func (f *Framework) BeforeEach() {
	// workaround for a bug in ginkgo.
	// https://github.com/onsi/ginkgo/issues/222
	f.cleanupHandle = AddCleanupAction(f.AfterEach)

	if !f.SkipNamespaceCreation {
		By(fmt.Sprintf("Creating namespace objects with basename %q", f.BaseName))

		namespaceLabels := map[string]string{
			"e2e-framework":                                  f.BaseName,
			"pod-security.kubernetes.io/enforce":             "privileged",
			"security.openshift.io/scc.podSecurityLabelSync": "false",
		}

		for idx, clientSet := range KubeClients {
			if ClusterIndex(idx) == ClusterA {
				// On the first cluster we let k8s generate a name for the namespace
				namespace := generateNamespace(clientSet, f.BaseName, namespaceLabels)
				f.Namespace = namespace.GetName()
				f.UniqueName = namespace.GetName()
				f.AddNamespacesToDelete(namespace)
				By(fmt.Sprintf("Generated namespace %q in cluster %q to execute the tests in", f.Namespace, TestContext.ClusterIDs[idx]))
			} else {
				// On the other clusters we use the same name to make tracing easier
				By(fmt.Sprintf("Creating namespace %q in cluster %q", f.Namespace, TestContext.ClusterIDs[idx]))
				f.CreateNamespace(clientSet, f.Namespace, namespaceLabels)
			}
		}
	} else {
		f.UniqueName = string(uuid.NewUUID())
	}
}

func DetectGlobalnet() {
	clusters := DynClients[ClusterA].Resource(schema.GroupVersionResource{
		Group:    "submariner.io",
		Version:  "v1",
		Resource: "clusters",
	}).Namespace(TestContext.SubmarinerNamespace)

	AwaitUntil("find Clusters to detect if Globalnet is enabled", func() (interface{}, error) {
		clusters, err := clusters.List(context.TODO(), metav1.ListOptions{})
		if apierrors.IsNotFound(err) {
			return nil, nil //nolint:nilnil // We want to repeat but let the checker known that nothing was found.
		}
		return clusters, err
	}, func(result interface{}) (bool, string, error) {
		if result == nil {
			return false, "No Cluster found", nil
		}

		clusterList := result.(*unstructured.UnstructuredList)
		if len(clusterList.Items) == 0 {
			return false, "No Cluster found", nil
		}

		for _, cluster := range clusterList.Items {
			cidrs, found, err := unstructured.NestedSlice(cluster.Object, "spec", "global_cidr")
			if err != nil {
				return false, "", err
			}

			if found && len(cidrs) > 0 {
				TestContext.GlobalnetEnabled = true
			}
		}

		return true, "", nil
	})
}

func InitNumClusterNodes() error {
	TestContext.NumNodesInCluster = map[ClusterIndex]int{}

	for i := range KubeClients {
		nodes, err := KubeClients[i].CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		TestContext.NumNodesInCluster[ClusterIndex(i)] = len(nodes.Items)
	}

	return nil
}

func fetchClusterIDs() {
	for i := range KubeClients {
		gatewayNodes := FindGatewayNodes(ClusterIndex(i))
		if len(gatewayNodes) == 0 {
			continue
		}

		name := "submariner-gateway"
		daemonSet := AwaitUntil(fmt.Sprintf("find %s DaemonSet for %q", name, TestContext.ClusterIDs[i]), func() (interface{}, error) {
			ds, err := KubeClients[i].AppsV1().DaemonSets(TestContext.SubmarinerNamespace).Get(context.TODO(), name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil, nil //nolint:nilnil // We want to repeat but let the checker known that nothing was found.
			}

			return ds, err
		}, func(result interface{}) (bool, string, error) {
			if result == nil {
				return false, "No DaemonSet found", nil
			}

			return true, "", nil
		}).(*appsv1.DaemonSet)

		const envVarName = "SUBMARINER_CLUSTERID"
		found := false

		for _, envVar := range daemonSet.Spec.Template.Spec.Containers[0].Env {
			if envVar.Name == envVarName {
				if TestContext.ClusterIDs[i] != envVar.Value {
					By(fmt.Sprintf("Setting new cluster ID %q, previous cluster ID was %q", envVar.Value, TestContext.ClusterIDs[i]))
					TestContext.ClusterIDs[i] = envVar.Value
				}

				found = true

				break
			}
		}

		Expect(found).To(BeTrue(), "Expected %q env var not found in DaemonSet %#v for kube context %q",
			envVarName, daemonSet, TestContext.ClusterIDs[i])
	}
}

func createKubernetesClient(restConfig *rest.Config) *kubeclientset.Clientset {
	clientSet, err := kubeclientset.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	// create scales getter, set GroupVersion and NegotiatedSerializer to default values
	// as they are required when creating a REST client.
	if restConfig.GroupVersion == nil {
		restConfig.GroupVersion = &schema.GroupVersion{}
	}

	if restConfig.NegotiatedSerializer == nil {
		restConfig.NegotiatedSerializer = scheme.Codecs
	}

	return clientSet
}

func createDynamicClient(restConfig *rest.Config) dynamic.Interface {
	clientSet, err := dynamic.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	return clientSet
}

func createRestConfig(kubeConfig, kubeContext string) *rest.Config {
	restConfig, err := loadConfig(kubeConfig, kubeContext)
	if err != nil {
		Errorf("Unable to load kubeconfig file %s for context %s, this is a non-recoverable error",
			TestContext.KubeConfig, kubeContext)
		Errorf("loadConfig err: %s", err.Error())
		fmt.Printf("Non-recoverable error: %s", err.Error())
		os.Exit(1)
	}

	restConfig.UserAgent = userAgentFunction()

	restConfig.QPS = TestContext.ClientQPS
	restConfig.Burst = TestContext.ClientBurst

	if TestContext.GroupVersion != nil {
		restConfig.GroupVersion = TestContext.GroupVersion
	}

	return restConfig
}

func deleteNamespace(client kubeclientset.Interface, namespaceName string) error {
	return client.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
}

// AfterEach deletes the namespace, after reading its events.
func (f *Framework) AfterEach() {
	RemoveCleanupAction(f.cleanupHandle)

	var nsDeletionErrors []error

	// Whether to delete namespace is determined by 3 factors: delete-namespace flag, delete-namespace-on-failure flag and the test result
	// if delete-namespace set to false, namespace will always be preserved.
	// if delete-namespace is true and delete-namespace-on-failure is false, namespace will be preserved if test failed.
	for ns := range f.namespacesToDelete {
		if err := f.deleteNamespaceFromAllClusters(ns); err != nil {
			nsDeletionErrors = append(nsDeletionErrors, err)
		}

		delete(f.namespacesToDelete, ns)
	}

	// Paranoia-- prevent reuse!
	f.Namespace = ""

	// if we had errors deleting, report them now.
	if len(nsDeletionErrors) != 0 {
		Failf(k8serrors.NewAggregate(nsDeletionErrors).Error())
	}
}

func (f *Framework) deleteNamespaceFromAllClusters(ns string) error {
	var errs []error

	for i, clientSet := range KubeClients {
		By(fmt.Sprintf("Deleting namespace %q on cluster %q", ns, TestContext.ClusterIDs[i]))

		if err := deleteNamespace(clientSet, ns); err != nil {
			switch {
			case apierrors.IsNotFound(err):
				Logf("Namespace %q was already deleted", ns)
			case apierrors.IsConflict(err):
				Logf("Namespace %v scheduled for deletion, resources being purged", ns)
			default:
				errs = append(errs, errors.WithMessagef(err, "Failed to delete namespace %q on cluster %q", ns, TestContext.ClusterIDs[i]))
			}
		}
	}

	return k8serrors.NewAggregate(errs)
}

// CreateNamespace creates a namespace for e2e testing.
func (f *Framework) CreateNamespace(clientSet *kubeclientset.Clientset,
	baseName string, labels map[string]string,
) *corev1.Namespace {
	ns := createTestNamespace(clientSet, baseName, labels)
	f.AddNamespacesToDelete(ns)

	return ns
}

func (f *Framework) AddNamespacesToDelete(namespaces ...*corev1.Namespace) {
	for _, ns := range namespaces {
		if ns == nil {
			continue
		}

		f.namespacesToDelete[ns.Name] = true
	}
}

func generateNamespace(client kubeclientset.Interface, baseName string, labels map[string]string) *corev1.Namespace {
	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-tests-%v-", baseName),
			Labels:       labels,
		},
	}

	namespace, err := client.CoreV1().Namespaces().Create(context.TODO(), namespaceObj, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "Error generating namespace %v", namespaceObj)

	return namespace
}

func createTestNamespace(client kubeclientset.Interface, name string, labels map[string]string) *corev1.Namespace {
	namespace := createNamespace(client, name, labels)
	return namespace
}

func createNamespace(client kubeclientset.Interface, name string, labels map[string]string) *corev1.Namespace {
	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	namespace, err := client.CoreV1().Namespaces().Create(context.TODO(), namespaceObj, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "Error creating namespace %v", namespaceObj)

	return namespace
}

// PatchString performs a REST patch operation for the given path and string value.
func PatchString(path, value string, patchFunc PatchFunc) {
	payload := []PatchStringValue{{
		Op:    "add",
		Path:  path,
		Value: value,
	}}

	doPatchOperation(payload, patchFunc)
}

// PatchInt performs a REST patch operation for the given path and int value.
func PatchInt(path string, value uint32, patchFunc PatchFunc) {
	payload := []PatchUInt32Value{{
		Op:    "add",
		Path:  path,
		Value: value,
	}}

	doPatchOperation(payload, patchFunc)
}

func doPatchOperation(payload interface{}, patchFunc PatchFunc) {
	payloadBytes, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())

	AwaitUntil("perform patch operation", func() (interface{}, error) {
		return nil, patchFunc(types.JSONPatchType, payloadBytes)
	}, NoopCheckResult)
}

func NoopCheckResult(interface{}) (bool, string, error) {
	return true, "", nil
}

// AwaitUntil periodically performs the given operation until the given CheckResultFunc returns true, an error, or a
// timeout is reached.
func AwaitUntil(opMsg string, doOperation DoOperationFunc, checkResult CheckResultFunc) interface{} {
	result, errMsg, err := AwaitResultOrError(opMsg, doOperation, checkResult)
	Expect(err).NotTo(HaveOccurred(), errMsg)

	return result
}

func AwaitResultOrError(opMsg string, doOperation DoOperationFunc, checkResult CheckResultFunc) (interface{}, string, error) {
	var finalResult interface{}
	var lastMsg string
	err := wait.PollImmediate(5*time.Second, time.Duration(TestContext.OperationTimeout)*time.Second, func() (bool, error) {
		result, err := doOperation()
		if err != nil {
			if IsTransientError(err, opMsg) {
				return false, nil
			}
			return false, err
		}

		ok, msg, err := checkResult(result)
		if err != nil {
			return false, err
		}

		if ok {
			finalResult = result
			return true, nil
		}

		lastMsg = msg
		return false, nil
	})

	errMsg := ""
	if err != nil {
		errMsg = "Failed to " + opMsg
		if lastMsg != "" {
			errMsg += ". " + lastMsg
		}
	}

	return finalResult, errMsg, err
}

func NestedString(obj map[string]interface{}, fields ...string) string {
	str, _, err := unstructured.NestedString(obj, fields...)
	Expect(err).To(Succeed())

	return str
}

func DetectProvider(ctx context.Context, cluster ClusterIndex, nodeName string) string {
	node, err := KubeClients[cluster].CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(node.Spec.ProviderID, ":")[0]
}
