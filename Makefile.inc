, := ,
USING = $(subst $(,), ,$(using))
_using = ${USING}
PRELOAD_IMAGES ?= submariner-gateway submariner-operator submariner-route-agent

### Tunable variables for affecting make commands ###
# Affecting multiple commands
TIMEOUT ?= 5m
export GLOBALNET PLUGIN SETTINGS TIMEOUT

# Specific to `clusters`
K8S_VERSION ?= 1.23
OLM_VERSION ?= v0.18.3
export K8S_VERSION OLM OLM_VERSION PROMETHEUS

# Specific to `deploy`
CABLE_DRIVER ?= libreswan
DEPLOYTOOL ?= operator
IMAGE_TAG ?= local
export CABLE_DRIVER DEPLOYTOOL IMAGE_TAG LIGHTHOUSE

# Specific to `e2e`
TESTDIR ?= test/e2e
LAZY_DEPLOY ?= true
export FOCUS LAZY_DEPLOY SKIP TESTDIR

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
$(call check_shipyard_file,Dockerfile.dapper)
$(call check_shipyard_file,Dockerfile.linting)

##### END GLOBAL FILES STALENESS CHECK #####

include $(SHIPYARD_DIR)/Makefile.images
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
PRELOAD_IMAGES += submariner-globalnet
endif

ifneq (,$(filter helm,$(_using)))
DEPLOYTOOL = helm
endif

ifneq (,$(filter bundle,$(_using)))
OLM = true
DEPLOYTOOL = bundle
PRELOAD_IMAGES += submariner-operator-index
endif

ifneq (,$(filter ocm,$(_using)))
OLM = true
DEPLOYTOOL = ocm
LIGHTHOUSE = true
PRELOAD_IMAGES += lighthouse-agent lighthouse-coredns submariner-operator-index
endif

ifneq (,$(filter prometheus,$(_using)))
PROMETHEUS = true
endif

ifneq (,$(filter ovn,$(_using)))
PRELOAD_IMAGES += submariner-networkplugin-syncer
endif

GO ?= go
export GO

# Shipyard provided targets

##### CLEANING TARGETS #####
.PHONY: clean clean-clusters clean-generated clean-buildx

# [clean] cleans everything (running clusters, generated files ...)
clean: clean-clusters clean-generated clean-buildx

# [clean-generated] removes files we generated, by removing all files and directories ignored by git
clean-generated:
	git clean -X -f -d

# [clean-clusters] removes running clusters
clean-clusters:
	$(SCRIPTS_DIR)/cleanup.sh
cleanup: clean-clusters

# [clean-buildx] removes the buildx builder, if any
clean-buildx:
	docker buildx version > /dev/null 2>&1 && \
	docker buildx use buildx_builder > /dev/null 2>&1 && \
	docker buildx rm

##### DEPLOYMENT& TESTING TARGETS #####
.PHONY: clusters deploy e2e upgrade-e2e deploy-latest

# [clusters] creates KIND clusters that can then be used to deploy Submariner
clusters:
	$(SCRIPTS_DIR)/$@.sh

# [deploy] deploys Submariner on KIND clusters
deploy: clusters preload-images
	$(SCRIPTS_DIR)/$@.sh

# [e2e] executes the project's end to end testing on the deployed KIND clusters
# Will call `deploy` if it detects no submariner running on a cluster
e2e:
	$(SCRIPTS_DIR)/e2e.sh $(E2E_ARGS)

upgrade-e2e: deploy-latest deploy e2e

# [deploy-latest] deploys the latest release
# This uses make deploy, but forcefully ignores images so that images
# are *not* rebuilt (we want to deploy the published images only)
deploy-latest:
	$(MAKE) -o images -o preload-images deploy SUBCTL_VERSION=latest IMAGE_TAG=subctl using=$(using)

##### LINTING TARGETS #####
.PHONY: gitlint golangci-lint markdownlint packagedoc-lint shellcheck yamllint

# [gitlint] validates the commits are valid
gitlint:
	if [ -r .gitlint ]; then \
	    gitlint --commits origin/$(BASE_BRANCH)..HEAD; \
	else \
	    gitlint --config $(SHIPYARD_DIR)/.gitlint --commits origin/$(BASE_BRANCH)..HEAD; \
	fi

# List of vendor/modules.txt files we need
VENDOR_MODULES := $(shell find . -name vendor -prune -o -name go.mod -printf "%h/vendor/modules.txt\n")

# [golangci-lint] validates Go code in the project
golangci-lint: $(VENDOR_MODULES)
ifneq (,$(shell find . -name '*.go'))
	golangci-lint version
	golangci-lint linters
	golangci-lint run --timeout 10m
else
	@echo 'There are no Go files to lint.'
endif

# [markdownlint] validates Markdown files in the project
markdownlint:
	md_ignored=($(patsubst %/modules.txt,%,$(VENDOR_MODULES))); \
	if [ -r .mdignore ]; then \
		md_ignored+=($$(< .mdignore)); \
	fi; \
	markdownlint -c .markdownlint.yml $${md_ignored[@]/#/-i } .

# [packagedoc-lint] checks that the package docs don’t include the SPDX header
packagedoc-lint:
	result=0; \
	for package in $$(find . -name vendor -prune -o -name \*.go -printf "%h\n" | sort -u); do \
		if $(GO) doc $$package | grep -q SPDX; then \
			echo $$package has an invalid package documentation; \
			result=1; \
		fi; \
	done 2>/dev/null; \
	exit $$result

# [shellcheck] validates your shell files
shellcheck:
# Only run shellcheck if there are files to check
ifneq (,$(SHELLCHECK_ARGS))
	shellcheck -x -P $${SCRIPTS_DIR} $(SHELLCHECK_ARGS)
	# https://github.com/koalaman/shellcheck/issues/1659
	! grep $$'\t' $(SHELLCHECK_ARGS)
else
	@echo 'There are no shell scripts to check; if this is incorrect, specify them in SHELLCHECK_ARGS.'
endif

# [yamllint] validates YAML files in the project
yamllint:
	yamllint --strict .

##### MISC TARGETS #####
.PHONY: backport post-mortem unit

# [backport] creates a PR to backport the PR identified by pr= to the release identified by release=
# This needs gh, and a GITHUB_TOKEN with repo and read:org
backport:
	$(SCRIPTS_DIR)/backport.sh $(release) $(pr)

# [post-mortem] prints a heap of information, to help in debugging failures on the KIND clusters
post-mortem:
	$(SCRIPTS_DIR)/post_mortem.sh

# [unit] executes the Go unit tests of the project
unit: $(VENDOR_MODULES)
	$(SCRIPTS_DIR)/unit_test.sh $(UNIT_TEST_ARGS)

vendor/modules.txt: go.mod
	$(GO) mod download
	$(GO) mod vendor
	$(GO) mod tidy -compat=1.17

%/vendor/modules.txt: %/go.mod
	cd $(patsubst %/vendor/,%,$(dir $@)) && \
	$(GO) mod download && \
	$(GO) mod vendor && \
	$(GO) mod tidy -compat=1.17

CODEOWNERS: CODEOWNERS.in
	$(SCRIPTS_DIR)/gen-codeowners

print-%: ; @echo $* = $($*)
