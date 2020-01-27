package defaults

import "time"

// Default values
const (
	// ClusterNameBase is the default prefix for all cluster names
	ClusterNameBase = "cluster"

	// PodCidrBase the default starting pod cidr for all the clusters
	PodCidrBase = "10.0.0.0"

	// PodCidrMask is the default mask for pod subnet
	PodCidrMask = "/14"

	// ServiceCidrBase the default starting service cidr for all the clusters
	ServiceCidrBase = "100.0.0.0"

	// ServiceCidrMask is the default mask for service subnet
	ServiceCidrMask = "/16"

	// NumWorkers is the number of worker nodes per cluster
	NumWorkers = 2

	// KindLogsDir is a default kind log files destination directory
	KindLogsDir = "output/logs"

	// KindConfigDir is a default kind config files destination directory
	KindConfigDir = "output/kind-clusters"

	// LocalKubeConfigDir is a default local workstation kubeconfig files destination directory
	LocalKubeConfigDir = "output/kube-config/local-dev"

	// LocalKubeConfigDir is a default  kubeconfig files destination directory if running inside container
	ContainerKubeConfigDir = "output/kube-config/container"

	// KubeAdminAPIVersion is a default version used by in kind configs
	KubeAdminAPIVersion = "kubeadm.k8s.io/v1beta2"
)

var (
	// WaitDurationResources is a default timeout for waiter functions
	WaitDurationResources = time.Duration(10) * time.Minute

	// WaitRetryPeriod is the amount oof time between retries for waiter functions
	WaitRetryPeriod = 2 * time.Second
)
