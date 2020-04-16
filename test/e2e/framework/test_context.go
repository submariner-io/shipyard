package framework

import (
	"flag"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
)

type contextArray []string

type TestContextType struct {
	KubeConfigs         []string // KubeConfigs provides an alternative to KubeConfig + KubeContexts
	KubeConfig          string
	KubeContexts        contextArray
	ClusterIDs          []string
	ReportDir           string
	ReportPrefix        string
	SubmarinerNamespace string
	ConnectionTimeout   uint
	ConnectionAttempts  uint
	OperationTimeout    uint
	GlobalnetEnabled    bool
	ClientQPS           float32
	ClientBurst         int
	GroupVersion        *schema.GroupVersion
}

func (contexts *contextArray) String() string {
	return strings.Join(*contexts, ",")
}

func (contexts *contextArray) Set(value string) error {
	*contexts = append(*contexts, value)
	return nil
}

var TestContext *TestContextType = &TestContextType{
	ClientQPS:   20,
	ClientBurst: 50,
}

func init() {
	flag.StringVar(&TestContext.KubeConfig, "kubeconfig", os.Getenv("KUBECONFIG"),
		"Path to kubeconfig containing embedded authinfo.")
	flag.Var(&TestContext.KubeContexts, "dp-context", "kubeconfig context for dataplane clusters (use several times).")
	flag.StringVar(&TestContext.ReportPrefix, "report-prefix", "", "Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.")
	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
	flag.StringVar(&TestContext.SubmarinerNamespace, "submariner-namespace", "submariner", "Namespace in which the submariner components are deployed.")
	flag.UintVar(&TestContext.ConnectionTimeout, "connection-timeout", 18, "The timeout in seconds per connection attempt when verifying communication between clusters.")
	flag.UintVar(&TestContext.ConnectionAttempts, "connection-attempts", 7, "The number of connection attempts when verifying communication between clusters.")
	flag.UintVar(&TestContext.OperationTimeout, "operation-timeout", 120, "The general operation timeout in seconds.")
}

func ValidateFlags(t *TestContextType) {
	if len(t.KubeConfig) == 0 && len(t.KubeConfigs) == 0 {
		klog.Fatalf("kubeconfig parameter or KUBECONFIG environment variable is required")
	}

	if len(t.KubeContexts) < 2 && len(t.KubeConfigs) < 2 {
		klog.Fatalf("several kubernetes contexts are necessary east, west, etc..")
	}
}
