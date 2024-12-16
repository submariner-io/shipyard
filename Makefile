BASE_BRANCH ?= devel
OCM_BASE_BRANCH ?= main
IMAGES ?= shipyard-dapper-base shipyard-linting nettest
MULTIARCH_IMAGES ?= nettest
EXTRA_PRELOAD_IMAGES := $(PRELOAD_IMAGES)
PLATFORMS ?= linux/amd64,linux/arm64
NON_DAPPER_GOALS += images multiarch-images
PLUGIN ?=

export BASE_BRANCH OCM_BASE_BRANCH

ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

ifneq (,$(filter ovn%,$(USING)))
SETTINGS ?= $(DAPPER_SOURCE)/.shipyard.e2e.ovn.yml
else
SETTINGS ?= $(DAPPER_SOURCE)/.shipyard.e2e.yml
endif

ifneq (,$(filter ovn-ic,$(USING)))
export OVN_IC = true
endif

export LAZY_DEPLOY = false

scale: SETTINGS = $(DAPPER_SOURCE)/.shipyard.scale.yml

include Makefile.inc

# In Shipyard we don't need to preload the dapper images, so override the default behavior.
ifneq ($(AIR_GAPPED),true)
override PRELOAD_IMAGES=nettest $(EXTRA_PRELOAD_IMAGES)
endif

# Prevent rebuilding images inside dapper since they're already built outside it in Shipyard's case
package/.image.shipyard-dapper-base: ;

# Project-specific targets go here
deploy: package/.image.nettest

e2e: clusters package/.image.nettest

else

# Not running in Dapper

export SCRIPTS_DIR=./scripts/shared

include Makefile.images
include Makefile.versions

# Shipyard-specific starts
# We need to ensure images, including the Shipyard base image, are updated
# before we start Dapper
clean-clusters cleanup cloud-prepare clusters deploy deploy-latest e2e golangci-lint post-mortem packagedoc-lint print-version scale unit upgrade-e2e: package/.image.shipyard-dapper-base
deploy deploy-latest e2e upgrade-e2e: package/.image.nettest

.DEFAULT_GOAL := lint
# Shipyard-specific ends

include Makefile.dapper

# Make sure linting goals have up-to-date linting image
$(LINTING_GOALS): package/.image.shipyard-linting

scale: scale-prereqs
scale-prereqs:
	@echo "We need to change some system parameters in order to continue, these may have a lasting effect on your system."
	@read -p "Do you wish to continue [y/N]?" response; \
	[[ "$${response,,}" =~ ^(yes|y)$$ ]] || exit 1
	# Increase general limits interfering with running multiple KIND clusters
	sudo sysctl -w fs.inotify.max_user_watches=1073741824
	sudo sysctl -w fs.inotify.max_user_instances=524288
	sudo sysctl -w kernel.pty.max=524288
	# Lower swappiness to avoid swapping unnecessarily, which would hurt the performance
	sudo sysctl -w vm.swappiness=5
	# Increase GC thresholds for IPv4 stack, otherwise we'll hit ARP table overflows
	sudo sysctl -w net.ipv4.neigh.default.gc_thresh1=2048
	sudo sysctl -w net.ipv4.neigh.default.gc_thresh2=4096
	sudo sysctl -w net.ipv4.neigh.default.gc_thresh3=8192
	# Increase open files limit. TODO: Find a way to make this change transient and not persistent
	echo "*	-	nofile	100000000" | sudo tee /etc/security/limits.d/shipyard.scale.conf

script-test: .dapper images
	-docker network create -d bridge kind
	$(RUN_IN_DAPPER) $(SCRIPT_TEST_ARGS)

.PHONY: script-test

endif
