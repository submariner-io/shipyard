package e2e

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/gobuffalo/packr/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	clustercmd "github.com/submariner-io/armada/cmd/cluster"
	"github.com/submariner-io/armada/pkg/cluster"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/deploy"
	"github.com/submariner-io/armada/pkg/image"
	"github.com/submariner-io/armada/pkg/wait"
	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

func CreateEnvironment(flags *clustercmd.CreateFlagpole, provider *kind.Provider) ([]*cluster.Config, error) {
	log.SetLevel(log.DebugLevel)
	box := packr.New("configs", "../../configs")

	targetClusters, err := clustercmd.GetTargetClusters(provider, flags)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(len(targetClusters))
	for _, cl := range targetClusters {
		go func(cl *cluster.Config) {
			err := cluster.Create(cl, provider, box, &wg)
			if err != nil {
				log.Fatal(err)
			}
		}(cl)
	}
	wg.Wait()

	wg.Add(len(targetClusters))
	for _, cl := range targetClusters {
		go func(cl *cluster.Config) {
			err := cluster.FinalizeSetup(cl, box, &wg)
			if err != nil {
				log.Fatal(err)
			}
		}(cl)
	}
	wg.Wait()
	return targetClusters, nil
}

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E test suite")
}

var _ = Describe("E2E Tests", func() {

	provider := kind.NewProvider(
		kind.ProviderWithLogger(kindcmd.NewLogger()),
	)

	var _ = AfterSuite(func() {
		_ = os.RemoveAll("./output")
	})
	usr, _ := user.Current()

	Context("Config creation", func() {
		It("Should create 2 clusters with flannel and overlapping cidrs", func() {
			flags := &clustercmd.CreateFlagpole{
				NumClusters: 2,
				Overlap:     true,
				Flannel:     true,
				Retain:      false,
				Wait:        5 * time.Minute,
			}

			clusters, err := CreateEnvironment(flags, provider)
			Ω(err).ShouldNot(HaveOccurred())

			cl1Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(1), provider)
			Ω(err).ShouldNot(HaveOccurred())
			cl2Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(2), provider)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(cl1Status).Should(BeTrue())
			Expect(cl2Status).Should(BeTrue())
			Expect(clusters).Should(Equal([]*cluster.Config{
				{
					Cni:                 "flannel",
					Name:                defaults.ClusterNameBase + strconv.Itoa(1),
					PodSubnet:           "10.0.0.0/14",
					ServiceSubnet:       "100.0.0.0/16",
					DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
					KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
					NumWorkers:          defaults.NumWorkers,
					KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
					Retain:              false,
					WaitForReady:        0,
				},
				{
					Cni:                 "flannel",
					Name:                defaults.ClusterNameBase + strconv.Itoa(2),
					PodSubnet:           "10.0.0.0/14",
					ServiceSubnet:       "100.0.0.0/16",
					DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(2) + ".local",
					KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
					NumWorkers:          defaults.NumWorkers,
					KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(2)),
					Retain:              false,
					WaitForReady:        0,
				},
			}))
		})

		It("Should create a third cluster with weave, kindest/node:v1.15.6 and tiller", func() {
			flags := &clustercmd.CreateFlagpole{
				NumClusters: 3,
				Weave:       true,
				Tiller:      true,
				ImageName:   "kindest/node:v1.15.6",
				Retain:      false,
				Wait:        5 * time.Minute,
			}

			clusters, err := CreateEnvironment(flags, provider)
			Ω(err).ShouldNot(HaveOccurred())

			ctx := context.Background()
			dockerCli, err := dockerclient.NewEnvClient()
			Ω(err).ShouldNot(HaveOccurred())

			containerFilter := filters.NewArgs()
			containerFilter.Add("name", defaults.ClusterNameBase+strconv.Itoa(3)+"-control-plane")
			container, err := dockerCli.ContainerList(ctx, dockertypes.ContainerListOptions{
				Filters: containerFilter,
				Limit:   1,
			})
			Ω(err).ShouldNot(HaveOccurred())
			image := container[0].Image
			cl3Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(3), provider)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(image).Should(Equal(flags.ImageName))
			Expect(cl3Status).Should(BeTrue())
			Expect(clusters).Should(Equal([]*cluster.Config{
				{
					Cni:                 "weave",
					Name:                defaults.ClusterNameBase + strconv.Itoa(3),
					PodSubnet:           "10.12.0.0/14",
					ServiceSubnet:       "100.3.0.0/16",
					DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(3) + ".local",
					KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
					NumWorkers:          defaults.NumWorkers,
					KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(3)),
					WaitForReady:        0,
					NodeImageName:       "kindest/node:v1.15.6",
					Retain:              false,
					Tiller:              true,
				},
			}))
		})

		It("Should not create a new cluster", func() {
			numClusters := 3
			for i := 1; i <= numClusters; i++ {
				clName := defaults.ClusterNameBase + strconv.Itoa(i)
				known, err := cluster.IsKnown(clName, provider)
				Ω(err).ShouldNot(HaveOccurred())
				if known {
					log.Infof("✔ Config with the name %q already exists.", clName)
				} else {
					Fail("Attempted to create a new cluster, but should have skipped as cluster already exists")
				}
			}
		})
	})

	Context("Deployment", func() {
		It("Should deploy nginx-demo to clusters 1 and 3", func() {
			log.SetLevel(log.DebugLevel)

			box := packr.New("configs", "../../configs")
			nginxDeploymentFile, err := box.Resolve("debug/nginx-demo-daemonset.yaml")
			Ω(err).ShouldNot(HaveOccurred())

			clusters := []string{"cluster1", "cluster3"}

			var activeDeployments []string
			var wg sync.WaitGroup
			wg.Add(len(clusters))
			for _, clName := range clusters {
				go func(clName string) {
					client, err := cluster.NewClient(clName)
					Ω(err).ShouldNot(HaveOccurred())

					err = deploy.Resources(clName, client, nginxDeploymentFile.String(), "Nginx")
					Ω(err).ShouldNot(HaveOccurred())

					err = wait.ForDaemonSetReady(clName, client, "default", "nginx-demo")
					Ω(err).ShouldNot(HaveOccurred())
					activeDeployments = append(activeDeployments, clName)
					wg.Done()
				}(clName)
			}
			wg.Wait()

			Expect(len(activeDeployments)).Should(Equal(2))

		})

		It("Should deploy netshoot to all 3 clusters", func() {
			log.SetLevel(log.DebugLevel)
			box := packr.New("configs", "../../configs")
			netshootDeploymentFile, err := box.Resolve("debug/netshoot-daemonset.yaml")
			Ω(err).ShouldNot(HaveOccurred())

			configFiles, err := ioutil.ReadDir(defaults.KindConfigDir)
			Ω(err).ShouldNot(HaveOccurred())

			var activeDeployments []string
			var wg sync.WaitGroup
			wg.Add(len(configFiles))
			for _, file := range configFiles {
				go func(file os.FileInfo) {
					clName := strings.FieldsFunc(file.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
					client, err := cluster.NewClient(clName)
					Ω(err).ShouldNot(HaveOccurred())

					err = deploy.Resources(clName, client, netshootDeploymentFile.String(), "Netshoot")
					Ω(err).ShouldNot(HaveOccurred())

					err = wait.ForDaemonSetReady(clName, client, "default", "netshoot")
					Ω(err).ShouldNot(HaveOccurred())
					activeDeployments = append(activeDeployments, clName)
					wg.Done()
				}(file)
			}
			wg.Wait()

			Expect(len(configFiles)).Should(Equal(3))
			Expect(len(activeDeployments)).Should(Equal(3))
		})
	})

	Context("Logs export", func() {
		It("Should export logs for clusters 1 and 2", func() {
			log.SetLevel(log.DebugLevel)

			for _, clName := range []string{"cluster1", "cluster2"} {
				err := provider.CollectLogs(clName, filepath.Join(defaults.KindLogsDir, clName))
				Ω(err).ShouldNot(HaveOccurred())
			}

			_, err := os.Stat(filepath.Join(defaults.KindLogsDir, "cluster1", "cluster1-control-plane"))
			Ω(err).ShouldNot(HaveOccurred())
			_, err = os.Stat(filepath.Join(defaults.KindLogsDir, "cluster2", "cluster2-control-plane"))
			Ω(err).ShouldNot(HaveOccurred())

		})
	})

	Context("Image loading", func() {
		ctx := context.Background()
		dockerCli, err := dockerclient.NewEnvClient()
		if err != nil {
			log.Fatal(err)
		}

		BeforeEach(func() {
			reader, err := dockerCli.ImagePull(ctx, "docker.io/library/alpine:edge", dockertypes.ImagePullOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, reader)
			Ω(err).ShouldNot(HaveOccurred())

			reader, err = dockerCli.ImagePull(ctx, "docker.io/library/alpine:latest", dockertypes.ImagePullOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, reader)
			Ω(err).ShouldNot(HaveOccurred())

			reader, err = dockerCli.ImagePull(ctx, "docker.io/library/nginx:stable-alpine", dockertypes.ImagePullOptions{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = io.Copy(os.Stdout, reader)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("Should load an image to all the clusters", func() {
			log.SetLevel(log.DebugLevel)

			var targetClusters []string
			configFiles, err := ioutil.ReadDir(defaults.KindConfigDir)
			Ω(err).ShouldNot(HaveOccurred())
			for _, configFile := range configFiles {
				clName := strings.FieldsFunc(configFile.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
				targetClusters = append(targetClusters, clName)
			}

			images := []string{"alpine:edge"}
			var nodesWithImage []nodes.Node
			for _, imageName := range images {
				localImageID, err := image.GetLocalID(ctx, dockerCli, imageName)
				Ω(err).ShouldNot(HaveOccurred())

				selectedNodes, err := image.GetNodesWithout(provider, imageName, localImageID, targetClusters)
				Ω(err).ShouldNot(HaveOccurred())
				Expect(len(selectedNodes)).Should(Equal(9))

				imageTarPath, err := image.Save(ctx, dockerCli, imageName)
				Ω(err).ShouldNot(HaveOccurred())
				defer os.RemoveAll(filepath.Dir(imageTarPath))

				log.Infof("loading image: %s to nodes: %s ...", imageName, selectedNodes)
				var wg sync.WaitGroup
				wg.Add(len(selectedNodes))
				for _, node := range selectedNodes {
					go func(node nodes.Node) {
						err = image.LoadToNode(imageTarPath, imageName, node, &wg)
						Ω(err).ShouldNot(HaveOccurred())
						nodesWithImage = append(nodesWithImage, node)
					}(node)
				}
				wg.Wait()
			}
			Expect(len(nodesWithImage)).Should(Equal(9))
		})
		It("Should load multiple images to cluster 1 and 3 only", func() {
			log.SetLevel(log.DebugLevel)

			images := []string{"nginx:stable-alpine", "alpine:latest"}
			clusters := []string{
				defaults.ClusterNameBase + strconv.Itoa(1),
				defaults.ClusterNameBase + strconv.Itoa(3),
			}

			var nodesWithImages []nodes.Node
			for _, imageName := range images {
				localImageID, err := image.GetLocalID(ctx, dockerCli, imageName)
				Ω(err).ShouldNot(HaveOccurred())

				selectedNodes, err := image.GetNodesWithout(provider, imageName, localImageID, clusters)
				Ω(err).ShouldNot(HaveOccurred())

				imageTarPath, err := image.Save(ctx, dockerCli, imageName)
				Ω(err).ShouldNot(HaveOccurred())
				defer os.RemoveAll(filepath.Dir(imageTarPath))

				log.Infof("loading image: %s to nodes: %s ...", imageName, selectedNodes)
				var wg sync.WaitGroup
				wg.Add(len(selectedNodes))
				for _, node := range selectedNodes {
					go func(node nodes.Node) {
						err = image.LoadToNode(imageTarPath, imageName, node, &wg)
						Ω(err).ShouldNot(HaveOccurred())
						nodesWithImages = append(nodesWithImages, node)
					}(node)
				}
				wg.Wait()
			}
			Expect(len(nodesWithImages)).Should(Equal(12))
		})
	})
	Context("Cluster deletion", func() {
		It("Should destroy clusters 1 and 3 only", func() {
			clusters := []string{defaults.ClusterNameBase + strconv.Itoa(1), defaults.ClusterNameBase + strconv.Itoa(3)}
			for _, clName := range clusters {
				known, err := cluster.IsKnown(clName, provider)
				Ω(err).ShouldNot(HaveOccurred())
				if known {
					err := cluster.Destroy(clName, provider)
					Ω(err).ShouldNot(HaveOccurred())
				}
			}

			cl1Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(1), provider)
			Ω(err).ShouldNot(HaveOccurred())
			cl2Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(2), provider)
			Ω(err).ShouldNot(HaveOccurred())
			cl3Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(3), provider)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(cl1Status).Should(BeFalse())
			Expect(cl2Status).Should(BeTrue())
			Expect(cl3Status).Should(BeFalse())
		})
		It("Should destroy all remaining clusters", func() {
			configFiles, err := ioutil.ReadDir(defaults.KindConfigDir)
			Ω(err).ShouldNot(HaveOccurred())

			for _, file := range configFiles {
				clName := strings.FieldsFunc(file.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
				err := cluster.Destroy(clName, provider)
				Ω(err).ShouldNot(HaveOccurred())
			}

			cl1Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(1), provider)
			Ω(err).ShouldNot(HaveOccurred())
			cl2Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(2), provider)
			Ω(err).ShouldNot(HaveOccurred())
			cl3Status, err := cluster.IsKnown(defaults.ClusterNameBase+strconv.Itoa(3), provider)
			Ω(err).ShouldNot(HaveOccurred())

			Expect(cl1Status).Should(BeFalse())
			Expect(cl2Status).Should(BeFalse())
			Expect(cl3Status).Should(BeFalse())
		})
	})
})
