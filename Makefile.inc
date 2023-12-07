, := ,
EXTRA_PRELOAD_IMAGES := $(PRELOAD_IMAGES)
USING = $(subst $(,), ,$(using))
_using = ${USING}

### Tunable variables for affecting make commands ###
# Affecting multiple commands
DEBUG_PRINT ?= true
PARALLEL ?= true
PROVIDER ?= kind
TIMEOUT ?= 5m
export AIR_GAPPED DEBUG_PRINT GLOBALNET PARALLEL PLUGIN PRELOAD_IMAGES PROVIDER SETTINGS TEST_ARGS TIMEOUT

# Specific to `deploy`
CABLE_DRIVER ?= libreswan
DEPLOYTOOL ?= operator
IMAGE_TAG ?= local
export CABLE_DRIVER DEPLOYTOOL IMAGE_TAG LIGHTHOUSE

# Specific to `e2e`
TESTDIR ?= test/e2e
LAZY_DEPLOY ?= true
SUBCTL_VERIFICATIONS ?= connectivity
export LAZY_DEPLOY SUBCTL_VERIFICATIONS TESTDIR

# Specific to `reload-images`
export RESTART ?= none

# Specific to `compile.sh`
export BUILD_DEBUG BUILD_UPX LDFLAGS

##### GLOBAL FILES STALENESS CHECK #####

define warn_shipyard_file
$(warning [31mYour $(1) and Shipyard's are different,[0m)
$(warning [31myou might run into issues. To avoid this, ensure[0m)
$(warning [31myour Shipyard image is up-to-date and run `make clean`.[0m)
endef

define _check_shipyard_file
$(info Checking if $(1) is up to date)
$(if $(shell cmp $(SHIPYARD_DIR)/$(1) $(1)),$(call warn_shipyard_file,$(1),))
endef

# Only check a file if it exists in the image and locally (not all images have all files)
check_shipyard_file=$(if $(wildcard $(SHIPYARD_DIR)/$(1)),$(if $(wildcard $(1)),$(call _check_shipyard_file,$(1)),),)

$(call check_shipyard_file,Makefile.dapper)
$(call check_shipyard_file,Makefile.shipyard)
$(call check_shipyard_file,Dockerfile.dapper)
$(call check_shipyard_file,Dockerfile.linting)

##### END GLOBAL FILES STALENESS CHECK #####

include $(SHIPYARD_DIR)/Makefile.clusters
include $(SHIPYARD_DIR)/Makefile.images
include $(SHIPYARD_DIR)/Makefile.linting
include $(SHIPYARD_DIR)/Makefile.versions

# Process extra flags from the `using=a,b,c` optional flag

ifneq (,$(filter libreswan,$(_using)))
CABLE_DRIVER = libreswan
else ifneq (,$(filter strongswan,$(_using)))
$(error strongSwan is no longer supported)
else ifneq (,$(filter wireguard,$(_using)))
# Wireguard requires kernel module install on the host
CABLE_DRIVER = wireguard
else ifneq (,$(filter vxlan,$(_using)))
CABLE_DRIVER = vxlan
endif

ifneq (,$(filter lighthouse,$(_using)))
LIGHTHOUSE = true
endif

ifneq (,$(filter globalnet,$(_using)))
GLOBALNET = true
endif

ifneq (,$(filter helm,$(_using)))
DEPLOYTOOL = helm
endif

ifneq (,$(filter bundle,$(_using)))
OLM = true
DEPLOYTOOL = bundle
endif

ifneq (,$(filter ocm,$(_using)))
OLM = true
DEPLOYTOOL = ocm
LIGHTHOUSE = true
endif

# Force running E2E with `subctl verify`
ifneq (,$(filter subctl-verify,$(_using)))
TESTDIR = non-existent-dir
endif

ifneq (,$(filter ocp,$(_using)))
IMAGE_TAG = subctl
endif

ifneq (,$(filter acm,$(_using)))
IMAGE_TAG = subctl
endif

ifneq (,$(filter load-balancer,$(_using)))
LOAD_BALANCER = true
endif

ifneq (,$(filter air-gap,$(_using)))
AIR_GAPPED = true
endif

ifeq ($(LIGHTHOUSE),true)
SUBCTL_VERIFICATIONS = service-discovery
endif

ifeq ($(PROVIDER),kind)
deploy: preload-images
endif

# Preload any images needed by air-gapped environment
ifeq ($(AIR_GAPPED),true)
override PRELOAD_IMAGES = $(EXTRA_PRELOAD_IMAGES) nettest \
	$(foreach image,gateway operator route-agent,submariner-$(image))

ifeq ($(GLOBALNET),true)
override PRELOAD_IMAGES += submariner-globalnet
endif
ifeq ($(LIGHTHOUSE),true)
override PRELOAD_IMAGES += lighthouse-agent lighthouse-coredns
endif
endif

GO ?= go
export GO

# Shipyard provided targets

##### CLEANING TARGETS #####
.PHONY: clean clean-generated clean-buildx

# [clean] cleans everything (running clusters, generated files ...)
clean: clean-clusters clean-generated clean-buildx

# [clean-generated] removes files we generated, by removing all files and directories ignored by git
clean-generated:
	git clean -X -f -d

# [clean-buildx] removes the buildx builder, if any
clean-buildx:
	docker buildx version > /dev/null 2>&1 && \
	docker buildx use buildx_builder > /dev/null 2>&1 && \
	docker buildx rm

##### DEPLOYMENT& TESTING TARGETS #####
.PHONY: cloud-prepare deploy e2e scale upgrade-e2e deploy-latest

# [cloud-prepare] can be run on a POC/testing cloud environment to prepare it before deploying Submariner
cloud-prepare:
	$(SCRIPTS_DIR)/$@.sh

# [deploy] deploys Submariner on KIND clusters
deploy: clusters cloud-prepare
	$(SCRIPTS_DIR)/$@.sh

# [e2e] executes the project's end to end testing on the deployed KIND clusters
# Will call `deploy` if it detects no submariner running on a cluster
e2e:
	$(SCRIPTS_DIR)/e2e.sh

# [scale] deploys KIND clusters with submariner for local scale testing
scale: PARALLEL = false
scale: deploy

upgrade-e2e: deploy-latest deploy e2e

# [deploy-latest] deploys the latest release
# This uses make deploy, but forcefully ignores images so that images
# are *not* rebuilt (we want to deploy the published images only)
deploy-latest:
	$(MAKE) -o images -o preload-images deploy SUBCTL_VERSION=latest IMAGE_TAG=subctl using=$(using)

##### MISC TARGETS #####
.PHONY: backport post-mortem unit

# [backport] creates a PR to backport the PR identified by pr= to the release identified by release=
# This needs gh, and a GITHUB_TOKEN with repo and read:org
backport:
	$(SCRIPTS_DIR)/backport.sh $(release) $(pr)

# [go-replace] allows anyone to quickly and easily set a Go replace
go-replace:
	go mod edit -replace=$(REPLACE)
	go mod tidy

# [post-mortem] prints a heap of information, to help in debugging failures on the KIND clusters
post-mortem:
	$(SCRIPTS_DIR)/post_mortem.sh

# [unit] executes the Go unit tests of the project
unit:
	$(SCRIPTS_DIR)/unit_test.sh

CODEOWNERS: CODEOWNERS.in
	$(SCRIPTS_DIR)/gen-codeowners

print-%: ; @echo $* = $($*)
