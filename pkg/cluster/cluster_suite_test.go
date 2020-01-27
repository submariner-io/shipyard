package cluster_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/defaults"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster test suite")
}

var _ = BeforeSuite(func() {
	defaults.WaitDurationResources = 1 * time.Minute
	defaults.WaitRetryPeriod = 200 * time.Millisecond
})
