BASE_BRANCH ?= devel
OCM_BASE_BRANCH ?= main
IMAGES ?= shipyard-dapper-base shipyard-linting nettest
MULTIARCH_IMAGES ?= nettest
PLATFORMS ?= linux/amd64,linux/arm64
NON_DAPPER_GOALS += images multiarch-images
SHELLCHECK_ARGS := $(shell find scripts -type f -exec awk 'FNR == 1 && /sh$$/ { print FILENAME }' {} +)
FOCUS ?=
SKIP ?=
PLUGIN ?=

export BASE_BRANCH OCM_BASE_BRANCH

ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

ifneq (,$(filter ovn,$(USING)))
SETTINGS ?= $(DAPPER_SOURCE)/.shipyard.e2e.ovn.yml
else
SETTINGS ?= $(DAPPER_SOURCE)/.shipyard.e2e.yml
endif

export LAZY_DEPLOY = false

include Makefile.inc

# Prevent rebuilding images inside dapper since they're already built outside it in Shipyard's case
package/.image.nettest package/.image.shipyard-dapper-base: ;

# Project-specific targets go here
deploy: package/.image.nettest

e2e: $(VENDOR_MODULES) clusters

else

# Not running in Dapper

export SCRIPTS_DIR=./scripts/shared

include Makefile.images
include Makefile.versions

# Shipyard-specific starts
# We need to ensure images, including the Shipyard base image, are updated
# before we start Dapper
clean-clusters cleanup clusters deploy deploy-latest e2e golangci-lint post-mortem print-version unit upgrade-e2e: package/.image.shipyard-dapper-base
deploy deploy-latest e2e upgrade-e2e: package/.image.nettest

.DEFAULT_GOAL := lint
# Shipyard-specific ends

include Makefile.dapper

# Make sure linting goals have up-to-date linting image
$(LINTING_GOALS): package/.image.shipyard-linting

script-test: .dapper images
	-docker network create -d bridge kind
	$(RUN_IN_DAPPER) $(SCRIPT_TEST_ARGS)

.PHONY: script-test

endif
