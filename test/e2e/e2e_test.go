package e2e

import (
	"testing"

	_ "github.com/submariner-io/shipyard/test/e2e/dataplane"
	_ "github.com/submariner-io/shipyard/test/e2e/example"
)

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
