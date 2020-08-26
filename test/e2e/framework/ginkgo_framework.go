package framework

import "github.com/onsi/ginkgo"

// NewFramework creates a test framework, under ginkgo
func NewFramework(baseName string) *Framework {
	f := NewBareFramework(baseName)

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)

	return f
}
