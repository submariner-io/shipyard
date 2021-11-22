BASE_BRANCH ?= devel
IMAGES ?= shipyard-dapper-base shipyard-linting nettest
NON_DAPPER_GOALS += images
SHELLCHECK_ARGS := scripts/shared/lib/*
FOCUS ?=
SKIP ?=
PLUGIN ?=

export BASE_BRANCH

ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

include Makefile.inc

ifneq (,$(filter ovn,$(_using)))
CLUSTER_SETTINGS_FLAG = --settings $(DAPPER_SOURCE)/.shipyard.e2e.ovn.yml
else
CLUSTER_SETTINGS_FLAG = --settings $(DAPPER_SOURCE)/.shipyard.e2e.yml
endif

override CLUSTERS_ARGS += $(CLUSTER_SETTINGS_FLAG)
override DEPLOY_ARGS += $(CLUSTER_SETTINGS_FLAG)
override E2E_ARGS += --nolazy_deploy cluster1

# Prevent rebuilding images inside dapper since thy're already built outside it in Shipyard's case
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
clusters deploy deploy-latest e2e golangci-lint post-mortem print-version unit upgrade-e2e: images

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
