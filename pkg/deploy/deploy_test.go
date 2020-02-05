package deploy_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const resourceNamespace = "test"

type failingClient struct {
	client.Client
}

func (c *failingClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return fmt.Errorf("Mock Create error")
}

var _ = BeforeSuite(func() {
	err := apiextscheme.AddToScheme(scheme.Scheme)
	Expect(err).To(Succeed())
})

var _ = Describe("Deploy tests", func() {
	var (
		resourcesFileName string
		client            client.Client
		deployErr         error
	)

	BeforeEach(func() {
		client = fake.NewFakeClientWithScheme(scheme.Scheme)
	})

	JustBeforeEach(func() {
		currentDir, err := os.Getwd()
		Expect(err).To(Succeed())

		raw, err := ioutil.ReadFile(filepath.Join(currentDir, "testdata", resourcesFileName))
		Expect(err).To(Succeed())

		deployErr = deploy.Resources("cluster1", client, string(raw), "Test")
	})

	When("provided with valid resource YAML content", func() {
		BeforeEach(func() {
			resourcesFileName = "resources.yaml"
		})

		It("should create the correct resource objects", func() {
			Expect(deployErr).To(Succeed())

			verifyConfigMap(client)
			verifyClusterRole(client)
			verifyClusterRoleBinding(client)
			verifyDaemonSet(client)
			verifyDeployment(client)
			verifyPod(client)
			verifyPodSecurityPolicy(client)
			verifyRole(client)
			verifyRoleBinding(client)
			verifyService(client)
			verifyServiceAccount(client)
		})

		When("creation of a resource fails", func() {
			BeforeEach(func() {
				client = &failingClient{fake.NewFakeClientWithScheme(scheme.Scheme)}
			})

			It("should return an error", func() {
				Expect(deployErr).To(HaveOccurred())
			})
		})

		When("resources already exist", func() {
			BeforeEach(func() {
				client = fake.NewFakeClientWithScheme(scheme.Scheme, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ConfigMap", Namespace: resourceNamespace},
					Data:       map[string]string{"foo": "bar"},
				}, &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ServiceAccount", Namespace: resourceNamespace},
				})
			})

			It("should return succcess", func() {
				Expect(deployErr).To(Succeed())
				verifyConfigMap(client)
				verifyServiceAccount(client)
			})
		})
	})

	When("provided with valid CRD YAML content", func() {
		BeforeEach(func() {
			resourcesFileName = "crds.yaml"
		})

		It("should create the correct CRD object", func() {
			Expect(deployErr).To(Succeed())

			actual := &apiextv1beta.CustomResourceDefinition{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: "config.acme.org", Namespace: ""}, actual)
			Expect(err).To(Succeed())

			Expect(actual.Spec.Group).To(Equal("crd.acme.org"))
			Expect(actual.Spec.Scope).To(Equal(apiextv1beta.ClusterScoped))
			Expect(actual.Spec.Versions).To(Equal([]apiextv1beta.CustomResourceDefinitionVersion{apiextv1beta.CustomResourceDefinitionVersion{Name: "v1"}}))
			Expect(actual.Spec.Names).To(Equal(apiextv1beta.CustomResourceDefinitionNames{
				Kind:     "TestConfiguration",
				Plural:   "TestConfigurations",
				Singular: "TestConfiguration",
				ListKind: "TestConfigurationList",
			}))
		})
	})

	When("provided with invalid YAML content", func() {
		BeforeEach(func() {
			resourcesFileName = "invalid.yaml"
		})

		It("should return an error", func() {
			Expect(deployErr).To(HaveOccurred())
		})
	})
})

func verifyConfigMap(client client.Client) {
	actual := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-ConfigMap", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())
	Expect(actual.Data).To(Equal(map[string]string{"foo": "bar"}))

}

func verifyClusterRole(client client.Client) {
	actual := &rbacv1.ClusterRole{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-ClusterRole", Namespace: ""}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Rules).To(Equal([]rbacv1.PolicyRule{rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{"acme.org"},
		Resources: []string{"pods"},
	}}))
}

func verifyClusterRoleBinding(client client.Client) {
	actual := &rbacv1.ClusterRoleBinding{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-ClusterRoleBinding", Namespace: ""}, actual)
	Expect(err).To(Succeed())

	Expect(actual.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "test-ClusterRole",
	}))

	Expect(actual.Subjects).To(Equal([]rbacv1.Subject{rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "test-ServiceAccount",
		Namespace: "test",
	}}))
}

func verifyDaemonSet(client client.Client) {
	actual := &appsv1.DaemonSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-DaemonSet", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Labels).To(Equal(map[string]string{"app": "test-controllers"}))
	Expect(actual.Spec).To(Equal(appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-controllers"}},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test-controllers"}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{corev1.Container{
					Name:    "foo",
					Image:   "quay.io/foo/bar",
					Command: []string{"/opt/bin/foo"},
					Env:     []corev1.EnvVar{corev1.EnvVar{Name: "FOO", Value: "bar"}},
				}},
			},
		},
	}))
}

func verifyDeployment(client client.Client) {
	actual := &appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-Deployment", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	replicas := int32(2)
	Expect(actual.Labels).To(Equal(map[string]string{"app": "test-controllers"}))
	Expect(actual.Spec).To(Equal(appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-controllers"}},
		Replicas: &replicas,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test-controllers"}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{corev1.Container{
					Name:    "foo",
					Image:   "quay.io/foo/bar",
					Command: []string{"/opt/bin/foo"},
					Env:     []corev1.EnvVar{corev1.EnvVar{Name: "FOO", Value: "bar"}},
				}},
			},
		},
	}))
}

func verifyPod(client client.Client) {
	actual := &corev1.Pod{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-Pod", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Spec).To(Equal(corev1.PodSpec{
		Containers: []corev1.Container{corev1.Container{
			Name:    "foo",
			Image:   "quay.io/foo/bar",
			Command: []string{"/opt/bin/foo"},
			Env:     []corev1.EnvVar{corev1.EnvVar{Name: "FOO", Value: "bar"}},
		}},
	}))
}

func verifyPodSecurityPolicy(client client.Client) {
	actual := &policyv1beta1.PodSecurityPolicy{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-PodSecurityPolicy", Namespace: ""}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Spec).To(Equal(policyv1beta1.PodSecurityPolicySpec{
		Privileged:          true,
		Volumes:             []policyv1beta1.FSType{policyv1beta1.FSType("configMap")},
		AllowedHostPaths:    []policyv1beta1.AllowedHostPath{policyv1beta1.AllowedHostPath{PathPrefix: "/etc/foo"}},
		RunAsUser:           policyv1beta1.RunAsUserStrategyOptions{Rule: "RunAsAny"},
		AllowedCapabilities: []corev1.Capability{corev1.Capability("NET_ADMIN")},
	}))
}

func verifyRole(client client.Client) {
	actual := &rbacv1.Role{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-Role", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Rules).To(Equal([]rbacv1.PolicyRule{rbacv1.PolicyRule{
		Verbs:     []string{"get", "list"},
		APIGroups: []string{"foo"},
		Resources: []string{"nodes", "services"},
	}}))
}

func verifyRoleBinding(client client.Client) {
	actual := &rbacv1.RoleBinding{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-RoleBinding", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	Expect(actual.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     "test-Role",
	}))

	Expect(actual.Subjects).To(Equal([]rbacv1.Subject{rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "test-ServiceAccount",
		Namespace: "test",
	}}))
}

func verifyService(client client.Client) {
	actual := &corev1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-Service", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())

	Expect(actual.Spec).To(Equal(corev1.ServiceSpec{
		ClusterIP:    "1.2.3.4",
		Type:         "ClusterIP",
		ExternalName: "foo",
	}))
}

func verifyServiceAccount(client client.Client) {
	actual := &corev1.ServiceAccount{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "test-ServiceAccount", Namespace: resourceNamespace}, actual)
	Expect(err).To(Succeed())
}

func TestDeployment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deploy test suite")
}
