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
clusters deploy e2e gitlint golangci-lint markdownlint nettest post-mortem unit: images

images: export SCRIPTS_DIR=./scripts/shared

images: package/.image.shipyard-dapper-base package/.image.nettest

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

NON_DAPPER_GOALS += prune-images
.PHONY: prune-images

include Makefile.dapper

endif
