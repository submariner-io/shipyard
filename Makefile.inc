, := ,
USING = $(subst $(,), ,$(using))
_using = ${USING}
PRELOAD_IMAGES ?= submariner-gateway submariner-operator submariner-route-agent

# Check Makefile.dapper freshness
ifneq (,$(shell cmp $(SHIPYARD_DIR)/Makefile.dapper Makefile.dapper))
$(warning [31mYour Makefile.dapper and Shipyard's are different,[0m)
$(warning [31myou might run into issues. To avoid this, ensure[0m)
$(warning [31myour Shipyard image is up-to-date and delete the[0m)
$(warning [31mlocal Makefile.dapper.[0m)
endif

include $(SHIPYARD_DIR)/Makefile.images
include $(SHIPYARD_DIR)/Makefile.versions

# Process extra flags from the `using=a,b,c` optional flag

ifneq (,$(filter libreswan,$(_using)))
override DEPLOY_ARGS += --cable_driver libreswan
else ifneq (,$(filter strongswan,$(_using)))
$(error strongSwan is no longer supported)
else ifneq (,$(filter wireguard,$(_using)))
# Wireguard requires kernel module install on the host
override DEPLOY_ARGS += --cable_driver wireguard
else ifneq (,$(filter vxlan,$(_using)))
override DEPLOY_ARGS += --cable_driver vxlan
endif

ifneq (,$(filter lighthouse,$(_using)))
override DEPLOY_ARGS += --service_discovery
endif

ifneq (,$(filter globalnet,$(_using)))
override CLUSTERS_ARGS += --globalnet
override DEPLOY_ARGS += --globalnet
override E2E_ARGS += --globalnet
PRELOAD_IMAGES += submariner-globalnet
endif

ifneq (,$(filter helm,$(_using)))
override DEPLOY_ARGS += --deploytool helm
endif

ifneq (,$(filter bundle,$(_using)))
override CLUSTERS_ARGS += --olm
override DEPLOY_ARGS += --deploytool bundle
PRELOAD_IMAGES += submariner-operator-index
endif

ifneq (,$(filter ocm,$(_using)))
override CLUSTERS_ARGS += --olm
override DEPLOY_ARGS += --service_discovery --deploytool ocm
PRELOAD_IMAGES += lighthouse-agent lighthouse-coredns submariner-operator-index
endif

ifneq (,$(filter prometheus,$(_using)))
override CLUSTERS_ARGS += --prometheus
endif

ifneq (,$(filter ovn,$(_using)))
PRELOAD_IMAGES += submariner-networkplugin-syncer
endif

ifdef FOCUS
override E2E_ARGS += --focus "$(FOCUS)"
endif

ifdef SKIP
override E2E_ARGS += --skip "$(SKIP)"
endif

ifdef E2E_TESTDIR
override E2E_ARGS += --testdir "$(E2E_TESTDIR)"
endif

ifdef PLUGIN
override DEPLOY_ARGS += --plugin $(PLUGIN)
override CLEANUP_ARGS += --plugin $(PLUGIN)
endif

ifdef SETTINGS
export SETTINGS
override CLUSTERS_ARGS += --settings $(SETTINGS)
override DEPLOY_ARGS += --settings $(SETTINGS)
endif

GO ?= go
export GO

# Shipyard provided targets

##### CLEANING TARGETS #####
.PHONY: clean clean-clusters clean-generated clean-buildx

# [clean] cleans everything (running clusters, generated files ...)
clean: clean-clusters clean-generated clean-buildx

# [clean-generated] removes files we generated
clean-generated:
	git clean -X -f

# [clean-clusters] removes running clusters
clean-clusters:
	$(SCRIPTS_DIR)/cleanup.sh $(CLEANUP_ARGS)
cleanup: clean-clusters

# [clean-buildx] removes the buildx builder, if any
clean-buildx:
	docker buildx version > /dev/null 2>&1 && \
	docker buildx use buildx_builder > /dev/null 2>&1 && \
	docker buildx rm

##### DEPLOYMENT& TESTING TARGETS #####
.PHONY: clusters preload-images deploy e2e upgrade-e2e deploy-latest

# [clusters] creates KIND clusters that can then be used to deploy Submariner
clusters:
	$(SCRIPTS_DIR)/clusters.sh $(CLUSTERS_ARGS)

preload-images: images
	set -e;\
	. $(SCRIPTS_DIR)/lib/deploy_funcs;\
	. $(SCRIPTS_DIR)/lib/debug_functions;\
	for image in $(PRELOAD_IMAGES); do\
		import_image quay.io/submariner/$${image};\
	done

# [reload-images] rebuilds and reload the images, optionally restarting with `restart` variable
reload-images: preload-images
	$(SCRIPTS_DIR)/reload_images.sh

# [deploy] deploys Submariner on KIND clusters
deploy: clusters preload-images
	$(SCRIPTS_DIR)/deploy.sh $(DEPLOY_ARGS)

# [e2e] executes the project's end to end testing on the deployed KIND clusters
# Will call `deploy` if it detects no submariner running on a cluster
e2e:
	$(SCRIPTS_DIR)/e2e.sh $(E2E_ARGS)

upgrade-e2e: deploy-latest deploy e2e

# [deploy-latest] deploys the latest release
# This uses make deploy, but forcefully ignores images so that images
# are *not* rebuilt (we want to deploy the published images only)
deploy-latest:
	curl -L get.submariner.io | VERSION=latest bash
	$(MAKE) -o images -o preload-images deploy SUBCTL=~/.local/bin/subctl DEPLOY_ARGS="$(DEPLOY_ARGS) --image_tag=subctl" using=$(using)

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
	$(GO) mod tidy

%/vendor/modules.txt: %/go.mod
	cd $(patsubst %/vendor/,%,$(dir $@)) && \
	$(GO) mod download && \
	$(GO) mod vendor && \
	$(GO) mod tidy

CODEOWNERS: CODEOWNERS.in
	$(SCRIPTS_DIR)/gen-codeowners

print-%: ; @echo $* = $($*)
