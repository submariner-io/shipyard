BASE_BRANCH ?= release-0.11
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
CLUSTER_SETTINGS_FLAG = --cluster_settings $(DAPPER_SOURCE)/scripts/cluster_settings.ovn
else
CLUSTER_SETTINGS_FLAG = --cluster_settings $(DAPPER_SOURCE)/scripts/cluster_settings
endif

override CLUSTERS_ARGS += $(CLUSTER_SETTINGS_FLAG)
override DEPLOY_ARGS += $(CLUSTER_SETTINGS_FLAG)
override E2E_ARGS += $(CLUSTER_SETTINGS_FLAG) --nolazy_deploy cluster1

TARGETS := $(shell ls -p scripts | grep -v -e /)

# Add any project-specific arguments here
$(TARGETS):
	./scripts/$@

.PHONY: $(TARGETS)

# Prevent rebuilding images inside dapper since thy're already built outside it in Shipyard's case
package/.image.nettest package/.image.shipyard-dapper-base: ;

# Project-specific targets go here
deploy: nettest

nettest: package/.image.nettest

e2e: $(VENDOR_MODULES) clusters

else

# Not running in Dapper

export SCRIPTS_DIR=./scripts/shared

include Makefile.images
include Makefile.versions

# Shipyard-specific starts
# We need to ensure images, including the Shipyard base image, are updated
# before we start Dapper
clusters deploy deploy-latest e2e golangci-lint nettest post-mortem print-version unit upgrade-e2e: images

.DEFAULT_GOAL := lint
# Shipyard-specific ends

# This removes all Submariner-provided images and all untagged images
# Use this to ensure you use current images
prune-images:
	docker images | grep -E '(admiral|lighthouse|nettest|shipyard|submariner|<none>)' | while read image tag hash _; do \
	    if [ "$$tag" != "<none>" ]; then \
	        docker rmi $$image:$$tag; \
	    else \
	        docker rmi $$hash; \
	    fi \
	done

backport:
	scripts/shared/backport.sh $(release) $(pr)

NON_DAPPER_GOALS += prune-images backport
.PHONY: prune-images

include Makefile.dapper

# Make sure linting goals have up-to-date linting image
$(LINTING_GOALS): package/.image.shipyard-linting

script-test: .dapper images
	-docker network create -d bridge kind
	$(RUN_IN_DAPPER) $(SCRIPT_TEST_ARGS)

.PHONY: script-test

endif
