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
	Context("ClusterNamesOrAll", testClusterNamesOrAll)
})

func testClusterName() {
	When("sent a number", func() {
		It("should return a cluster name", func() {
			Expect("cluster42").To(Equal(ClusterName(42)))
		})
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

	When("directory doesn't exist", func() {
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("directory exists", func() {
		BeforeEach(createStubDirectory)

		AfterEach(deleteStubDirectory)

		When("the directory is empty", func() {
			It("should return an empty slice", func() {
				Expect(len(clusters)).To(Equal(0))
			})
		})

		When("the directory has files", func() {
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

func testClusterNamesOrAll() {
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
		clusters = ClusterNamesOrAll(sentClusters)
	})

	When("cluster names are sent", func() {
		BeforeEach(func() {
			sentClusters = []string{"cluster13", "cluster76"}
		})
		It("should return the sent clusters", func() {
			Expect(clusters).To(ConsistOf(sentClusters))
		})
	})

	When("directory doesn't exist", func() {
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

	When("directory exists", func() {
		BeforeEach(createStubDirectory)

		AfterEach(deleteStubDirectory)

		When("the directory is empty", func() {
			It("should return an empty slice", func() {
				Expect(len(clusters)).To(Equal(0))
			})
		})

		When("the directory has files", func() {
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
