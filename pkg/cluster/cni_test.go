package cluster_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("CNI tests", func() {
	box := packr.New("configs", "../../configs")

	Context("GenerateWeaveDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateWeaveDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/14"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "weave_deployment.golden")
		})
	})

	Context("GenerateFlannelDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateFlannelDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/8"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "flannel_deployment.golden")
		})
	})

	Context("GenerateCalicoDeploymentFile function", func() {
		It("should generate the correct deployment yaml", func() {
			actual, err := cluster.GenerateCalicoDeploymentFile(&cluster.Config{PodSubnet: "1.2.3.4/16"}, box)
			Expect(err).To(Succeed())

			verifyDeployment(actual, "calico_deployment.golden")
		})
	})

	Context("CNI deployment", func() {
		testDeployCNIs(box)
	})
})

type interceptorMap map[reflect.Type]func(obj runtime.Object)

type interceptingClient struct {
	client.Client
	getInterceptors interceptorMap
}

func (c *interceptingClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	err := c.Client.Get(ctx, key, obj)
	if err != nil {
		return err
	}

	f, found := c.getInterceptors[reflect.TypeOf(obj)]
	if found {
		f(obj)
	}

	return nil
}

func testDeployCNIs(box *packr.Box) {
	var (
		client    client.Client
		config    *cluster.Config
		deployErr error
	)

	BeforeEach(func() {
		config = &cluster.Config{
			Name:      "east",
			PodSubnet: "1.2.3.4/8",
		}
	})

	JustBeforeEach(func() {
		client = &interceptingClient{Client: fake.NewFakeClientWithScheme(scheme.Scheme),
			getInterceptors: interceptorMap{reflect.TypeOf(&appsv1.DaemonSet{}): func(obj runtime.Object) {
				d := obj.(*appsv1.DaemonSet)
				d.Status.DesiredNumberScheduled = 1
				d.Status.NumberReady = d.Status.DesiredNumberScheduled
			}, reflect.TypeOf(&appsv1.Deployment{}): func(obj runtime.Object) {
				d := obj.(*appsv1.Deployment)
				d.Status.ReadyReplicas = *d.Spec.Replicas
			}},
		}

		deployErr = cluster.DeployCni(config, box, client)
	})

	When("the weave CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Weave
		})

		It("should create the weave DaemonSet", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSet("weave-net", client)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("docker.io/weaveworks/weave-kube:2.6.0"))
			Expect(ds.Spec.Template.Spec.Containers[0].Env[1].Value).Should(Equal(config.PodSubnet))
		})
	})

	When("the flannel CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Flannel
		})

		It("should create the flannel DaemonSet and ConfigMap", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSet("kube-flannel-ds-amd64", client)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("quay.io/coreos/flannel:v0.11.0-amd64"))

			cm := &corev1.ConfigMap{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: "kube-flannel-cfg", Namespace: "kube-system"}, cm)
			Expect(err).To(Succeed())
			Expect(cm.Data["net-conf.json"]).To(ContainSubstring(config.PodSubnet))
		})
	})

	When("the calico CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Calico
		})

		It("should create the calico DaemonSet and Deployment", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSet("calico-node", client)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("calico/node:v3.9.3"))
			Expect(ds.Spec.Template.Spec.Containers[0].Env[8].Value).Should(Equal(config.PodSubnet))

			getDeployment("calico-kube-controllers", client)
		})
	})

	When("an invalid CNI name is specified", func() {
		BeforeEach(func() {
			config.Cni = "bogus"
		})

		It("should return an error", func() {
			Expect(deployErr).To(HaveOccurred())
		})
	})
}

func getDaemonSet(name string, client client.Client) *appsv1.DaemonSet {
	daemonSet := &appsv1.DaemonSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "kube-system"}, daemonSet)
	Expect(err).To(Succeed())
	return daemonSet
}

func getDeployment(name string, client client.Client) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "kube-system"}, deployment)
	Expect(err).To(Succeed())
	return deployment
}

func verifyDeployment(actualDeploymentContents string, expDeploymentFileName string) {
	currentDir, err := os.Getwd()
	Expect(err).To(Succeed())
	configDir := filepath.Join(currentDir, "testdata/cni")

	expDeploymentFilePath := filepath.Join(configDir, expDeploymentFileName)
	expectedContents, err := ioutil.ReadFile(expDeploymentFilePath)
	Expect(err).To(Succeed())

	Expect(string(actualDeploymentContents)).Should(Equal(string(expectedContents)))
}
