package deploy_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestDeployment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deploy test suite")
}

var _ = Describe("Deploy tests", func() {

	box := packr.New("configs", "../../configs")

	Context("Deployment tests", func() {
		It("Should deploy weave resources", func() {

			cl := &cluster.Config{
				Name:      "cl1",
				PodSubnet: "1.2.3.4/8",
			}

			clientSet := testclient.NewSimpleClientset()

			deployfile, err := cluster.GenerateWeaveDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())

			err = deploy.Resources(cl.Name, clientSet, deployfile, "Weave")
			Ω(err).ShouldNot(HaveOccurred())

			result, err := clientSet.AppsV1().DaemonSets("kube-system").Get("weave-net", metav1.GetOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Printf("Name: %s, Value: %s", result.Spec.Template.Spec.Containers[0].Env[1].Name, result.Spec.Template.Spec.Containers[0].Env[1].Value)

			Expect(result.Spec.Template.Spec.Containers[0].Image).Should(Equal("docker.io/weaveworks/weave-kube:2.6.0"))
			Expect(result.Spec.Template.Spec.Containers[0].Env[1].Value).Should(Equal(cl.PodSubnet))
		})
		It("Should deploy flannel resources", func() {

			cl := &cluster.Config{
				Name:      "cl1",
				PodSubnet: "1.2.3.4/16",
			}

			clientSet := testclient.NewSimpleClientset()

			deployfile, err := cluster.GenerateFlannelDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())

			err = deploy.Resources(cl.Name, clientSet, deployfile, "Flannel")
			Ω(err).ShouldNot(HaveOccurred())

			result, err := clientSet.CoreV1().ConfigMaps("kube-system").Get("kube-flannel-cfg", metav1.GetOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Print(result.Data["net-conf.json"])
			contains := strings.Contains(result.Data["net-conf.json"], cl.PodSubnet)
			Expect(contains).Should(BeTrue())
		})
		It("Should deploy calico resources", func() {

			cl := &cluster.Config{
				Name:      "cl1",
				PodSubnet: "1.2.3.4/4",
			}

			clientSet := testclient.NewSimpleClientset()

			deployfile, err := cluster.GenerateCalicoDeploymentFile(cl, box)
			Ω(err).ShouldNot(HaveOccurred())

			err = deploy.Resources(cl.Name, clientSet, deployfile, "Calico")
			Ω(err).ShouldNot(HaveOccurred())

			result, err := clientSet.AppsV1().DaemonSets("kube-system").Get("calico-node", metav1.GetOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Printf("Name: %s, Value: %s", result.Spec.Template.Spec.Containers[0].Env[8].Name, result.Spec.Template.Spec.Containers[0].Env[8].Value)

			Expect(result.Spec.Template.Spec.Containers[0].Image).Should(Equal("calico/node:v3.9.3"))
			Expect(result.Spec.Template.Spec.Containers[0].Env[8].Value).Should(Equal(cl.PodSubnet))
		})
	})
})
