package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/defaults"
)

const stubConfigDir = "utils.test.dir"

var origDir string

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils test suite")
}

var _ = Describe("Utils tests", func() {
	Context("ClusterName", testClusterName)
	Context("ClusterNamesFromFiles", testClusterNamesFromFiles)
	Context("DetermineClusterNames", testDetermineClusterNames)
})

func testClusterName() {
	It("should return the correct cluster name for the given number", func() {
		Expect("cluster42").To(Equal(ClusterName(42)))
	})
}

func stubDirectory() {
	origDir = defaults.KindConfigDir
	defaults.KindConfigDir = stubConfigDir
}

func restoreDirectory() {
	defaults.KindConfigDir = origDir
}

func createStubDirectory() {
	Expect(os.Mkdir(stubConfigDir, 0755)).To(Succeed())
}

func deleteStubDirectory() {
	Expect(os.RemoveAll(stubConfigDir)).To(Succeed())
}

func generateClusterFiles(expectedClusters []string) {
	for _, cluster := range expectedClusters {
		fileName := fmt.Sprintf("kind-config-%s.yaml", cluster)
		_, err := os.Create(filepath.Join(stubConfigDir, fileName))
		Expect(err).To(Succeed())
	}
}

func testClusterNamesFromFiles() {
	var (
		clusters []string
		err      error
	)

	BeforeEach(stubDirectory)

	AfterEach(restoreDirectory)

	JustBeforeEach(func() {
		clusters, err = ClusterNamesFromFiles()
	})

	When("the directory doesn't exist", func() {
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("the directory exists", func() {
		BeforeEach(createStubDirectory)

		AfterEach(deleteStubDirectory)

		Context("and is empty", func() {
			It("should return an empty slice", func() {
				Expect(len(clusters)).To(Equal(0))
			})
		})

		Context("and contains files", func() {
			var expectedClusters = []string{"cluster1", "cluster2", "cluster42"}
			BeforeEach(func() {
				generateClusterFiles(expectedClusters)
			})

			It("should return the cluster names", func() {
				Expect(clusters).To(ConsistOf(expectedClusters))
			})
		})
	})
}

func testDetermineClusterNames() {
	var (
		clusters     []string
		sentClusters []string
	)

	BeforeEach(func() {
		stubDirectory()
		sentClusters = nil
	})

	AfterEach(restoreDirectory)

	JustBeforeEach(func() {
		clusters = DetermineClusterNames(sentClusters)
	})

	When("cluster names are provided", func() {
		BeforeEach(func() {
			sentClusters = []string{"cluster13", "cluster76"}
		})
		It("should return the provided clusters", func() {
			Expect(clusters).To(ConsistOf(sentClusters))
		})
	})

	When("the directory doesn't exist", func() {
		var (
			logFatalCalled bool
			origLogFatal   func(args ...interface{})
		)

		BeforeEach(func() {
			logFatalCalled = false
			origLogFatal = logFatal
			logFatal = func(args ...interface{}) {
				logFatalCalled = true
			}
		})

		AfterEach(func() {
			logFatal = origLogFatal
		})

		It("should call log fatal", func() {
			Expect(logFatalCalled).To(Equal(true))
		})
	})

	When("the directory exists", func() {
		BeforeEach(createStubDirectory)

		AfterEach(deleteStubDirectory)

		Context("and is empty", func() {
			It("should return an empty slice", func() {
				Expect(len(clusters)).To(Equal(0))
			})
		})

		Context("and contains files", func() {
			var expectedClusters = []string{"cluster1", "cluster2", "cluster42"}
			BeforeEach(func() {
				generateClusterFiles(expectedClusters)
			})

			It("should return the cluster names", func() {
				Expect(clusters).To(ConsistOf(expectedClusters))
			})
		})
	})
}
