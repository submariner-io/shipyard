package cluster_test

import (
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	createclustercmd "github.com/submariner-io/armada/cmd/armada/create/cluster"
	"github.com/submariner-io/armada/pkg/cluster"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/defaults"
)

var _ = Describe("config tests", func() {
	Context("Default flags", func() {
		It("Should populate config with correct default values", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				Kindnet: true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())

			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "kindnet",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        5 * time.Minute,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
	})
	Context("Custom flags", func() {
		It("Should set KubeAdminAPIVersion to kubeadm.k8s.io/v1beta1", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				ImageName: "kindest/node:v1.11.1",
				Weave:     true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "weave",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta1",
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        0,
				NodeImageName:       "kindest/node:v1.11.1",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should set KubeAdminAPIVersion to kubeadm.k8s.io/v1beta2", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				ImageName: "kindest/node:v1.16.3",
				Kindnet:   true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "kindnet",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        5 * time.Minute,
				NodeImageName:       "kindest/node:v1.16.3",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should set KubeAdminAPIVersion to kubeadm.k8s.io/v1beta2 if image name is empty", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				ImageName: "",
				Kindnet:   true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "kindnet",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: "kubeadm.k8s.io/v1beta2",
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        5 * time.Minute,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should return error with invalid node image name", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				ImageName: "kindest/node:1.16.3",
			}
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, true, true, true, 0)
			Ω(err).Should(HaveOccurred())
			Expect(got).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should set Cni to weave and WaitForReady should be zero", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				Weave: true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "weave",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        0,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should set Cni to calico and WaitForReady should be zero", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				Calico: true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "calico",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        0,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should set Cni to flannel and WaitForReady should be zero", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				Flannel: true,
			}
			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())
			cni := createclustercmd.GetCniFromFlags(flags)
			got, err := cluster.PopulateConfig(1, flags.ImageName, cni, false, false, false, 5*time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(got).Should(Equal(&cluster.Config{
				Cni:                 "flannel",
				Name:                defaults.ClusterNameBase + strconv.Itoa(1),
				PodSubnet:           "10.4.0.0/14",
				ServiceSubnet:       "100.1.0.0/16",
				DNSDomain:           defaults.ClusterNameBase + strconv.Itoa(1) + ".local",
				KubeAdminAPIVersion: defaults.KubeAdminAPIVersion,
				NumWorkers:          defaults.NumWorkers,
				KubeConfigFilePath:  filepath.Join(usr.HomeDir, ".kube", "kind-config-"+defaults.ClusterNameBase+strconv.Itoa(1)),
				WaitForReady:        0,
				NodeImageName:       "",
				Retain:              false,
				Tiller:              false,
			}))
		})
		It("Should create configs for 2 clusters with flannel and overlapping cidrs", func() {
			flags := &createclustercmd.CreateClusterFlagpole{
				Flannel:     true,
				Overlap:     true,
				NumClusters: 2,
			}

			usr, err := user.Current()
			Ω(err).ShouldNot(HaveOccurred())

			var clusters []*cluster.Config
			for i := 1; i <= flags.NumClusters; i++ {
				cni := createclustercmd.GetCniFromFlags(flags)
				cl, err := cluster.PopulateConfig(i, flags.ImageName, cni, false, true, true, 5*time.Minute)
				Ω(err).ShouldNot(HaveOccurred())
				clusters = append(clusters, cl)
			}
			Expect(len(clusters)).Should(Equal(2))
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
					WaitForReady:        0,
					NodeImageName:       "",
					Retain:              false,
					Tiller:              true,
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
					WaitForReady:        0,
					NodeImageName:       "",
					Retain:              false,
					Tiller:              true,
				},
			}))
		})
	})
})
