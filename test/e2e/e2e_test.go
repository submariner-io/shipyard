package e2e

import (
	"testing"

	_ "github.com/submariner-io/shipyard/test/e2e/example"
	_ "github.com/submariner-io/submariner/test/e2e/framework"
)

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
