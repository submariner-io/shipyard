package cluster_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	appsv1 "k8s.io/api/apps/v1"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	fakeApiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
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

func testDeployCNIs(box *packr.Box) {
	var (
		clientSet       kubernetes.Interface
		apiExtClientSet apiextclientset.Interface
		config          *cluster.Config
		stopCh          chan struct{}
		deployErr       error
	)

	BeforeEach(func() {
		clientSet = fake.NewSimpleClientset()
		apiExtClientSet = fakeApiext.NewSimpleClientset()
		stopCh = make(chan struct{})

		config = &cluster.Config{
			Name:      "east",
			PodSubnet: "1.2.3.4/8",
		}
	})

	AfterEach(func() {
		close(stopCh)
	})

	JustBeforeEach(func() {
		factory := informers.NewSharedInformerFactory(clientSet, 0)
		factory.Apps().V1().Deployments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				d := obj.(*appsv1.Deployment)
				d.Status.ReadyReplicas = *d.Spec.Replicas
				_, _ = clientSet.AppsV1().Deployments(d.Namespace).Update(d)
			},
		})

		factory.Apps().V1().DaemonSets().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				d := obj.(*appsv1.DaemonSet)
				d.Status.DesiredNumberScheduled = 1
				d.Status.NumberReady = d.Status.DesiredNumberScheduled
				_, _ = clientSet.AppsV1().DaemonSets(d.Namespace).Update(d)
			},
		})

		factory.Start(stopCh)

		deployErr = cluster.DeployCni(config, box, clientSet, apiExtClientSet)
	})

	When("the weave CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Weave
		})

		It("should create the weave-net DaemonSet", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSetByLabel("name=weave-net", clientSet)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("docker.io/weaveworks/weave-kube:2.6.0"))
			Expect(ds.Spec.Template.Spec.Containers[0].Env[1].Value).Should(Equal(config.PodSubnet))
		})
	})

	When("the flannel CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Flannel
		})

		It("should create the flannel DaemonSet and kube-flannel-cfg ConfigMap", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSetByLabel("app=flannel", clientSet)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("quay.io/coreos/flannel:v0.11.0-amd64"))

			cm, err := clientSet.CoreV1().ConfigMaps("kube-system").Get("kube-flannel-cfg", metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(cm.Data["net-conf.json"]).To(ContainSubstring(config.PodSubnet))
		})
	})

	When("the calico CNI is specified", func() {
		BeforeEach(func() {
			config.Cni = cluster.Calico
		})

		It("should create the calico-node DaemonSet and the calico-kube-controllers Deployment", func() {
			Expect(deployErr).To(Succeed())
			ds := getDaemonSetByLabel("k8s-app=calico-node", clientSet)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal("calico/node:v3.9.3"))
			Expect(ds.Spec.Template.Spec.Containers[0].Env[8].Value).Should(Equal(config.PodSubnet))

			getDeploymentByLabel("k8s-app=calico-kube-controllers", clientSet)
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

func getDaemonSetByLabel(label string, clientSet kubernetes.Interface) *appsv1.DaemonSet {
	list, err := clientSet.AppsV1().DaemonSets("kube-system").List(metav1.ListOptions{LabelSelector: label})
	Expect(err).To(Succeed())
	Expect(list.Items).To(HaveLen(1))
	return &list.Items[0]
}

func getDeploymentByLabel(label string, clientSet kubernetes.Interface) *appsv1.Deployment {
	list, err := clientSet.AppsV1().Deployments("kube-system").List(metav1.ListOptions{LabelSelector: label})
	Expect(err).To(Succeed())
	Expect(list.Items).To(HaveLen(1))
	return &list.Items[0]
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
