ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

CLUSTER_SETTINGS_FLAG = --cluster_settings $(DAPPER_SOURCE)/scripts/cluster_settings
override CLUSTERS_ARGS += $(CLUSTER_SETTINGS_FLAG)
override DEPLOY_ARGS += $(CLUSTER_SETTINGS_FLAG)
override E2E_ARGS += $(CLUSTER_SETTINGS_FLAG) --nolazy_deploy cluster1

include Makefile.inc

TARGETS := $(shell ls -p scripts | grep -v -e /)

# Add any project-specific arguments here
$(TARGETS):
	./scripts/$@

.PHONY: $(TARGETS)

# Project-specific targets go here
deploy: nettest

nettest: package/.image.nettest

e2e: vendor/modules.txt clusters

shellcheck:
# SC2154 is excluded to avoid false positives based on our use of global variables
	shellcheck -e SC2154 scripts/shared/lib/*

else

# Not running in Dapper

include Makefile.images
include Makefile.versions

# Shipyard-specific starts
clusters deploy e2e gitlint golangci-lint markdownlint nettest post-mortem unit-test: images

images: export SCRIPTS_DIR=./scripts/shared

images: package/.image.shipyard-dapper-base

.DEFAULT_GOAL := lint
# Shipyard-specific ends

include Makefile.dapper

endif
