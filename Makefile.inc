, := ,
_using = $(subst $(,), ,$(using))
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

ifneq (,$(filter olm,$(_using)))
override CLUSTERS_ARGS += --olm
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

# Shipyard provided targets

# clean cleans everything (running clusters, generated files ...)
clean: clean-clusters clean-generated

# clean-generated removes files we generated
clean-generated:
	git clean -X -f

# clean-clusters removes running clusters
clean-clusters:
	$(SCRIPTS_DIR)/cleanup.sh $(CLEANUP_ARGS)
cleanup: clean-clusters

clusters:
	$(SCRIPTS_DIR)/clusters.sh $(CLUSTERS_ARGS)

preload-images: images
	set -e;\
	. $(SCRIPTS_DIR)/lib/deploy_funcs;\
	. $(SCRIPTS_DIR)/lib/debug_functions;\
	for image in $(PRELOAD_IMAGES); do\
		import_image quay.io/submariner/$${image};\
	done

deploy: clusters preload-images
	$(SCRIPTS_DIR)/deploy.sh $(DEPLOY_ARGS)

# Will call `deploy` if it detects no submariner running on a cluster
e2e:
	$(SCRIPTS_DIR)/e2e.sh $(E2E_ARGS)

upgrade-e2e: deploy-latest deploy e2e

# Deploys the latest release
# This uses make deploy, but forcefully ignores images so that images
# are *not* rebuilt (we want to deploy the published images only)
deploy-latest:
	curl -L get.submariner.io | VERSION=latest bash
	$(MAKE) -o images -o preload-images deploy SUBCTL=~/.local/bin/subctl DEV_VERSION=latest CUTTING_EDGE=latest VERSION=latest DEPLOY_ARGS="$(DEPLOY_ARGS) --image_tag=subctl" using=$(using)

gitlint:
	gitlint --commits origin/$(BASE_BRANCH)..HEAD

golangci-lint: vendor/modules.txt
	golangci-lint linters
	golangci-lint run --timeout 10m

lint: gitlint golangci-lint markdownlint yamllint

markdownlint:
	markdownlint markdownlint -c .markdownlint.yml -i vendor .

yamllint:
	yamllint .

release:
	$(SCRIPTS_DIR)/release.sh $(IMAGES) $(RELEASE_ARGS)

post-mortem:
	$(SCRIPTS_DIR)/post_mortem.sh

unit: vendor/modules.txt
	$(SCRIPTS_DIR)/unit_test.sh $(UNIT_TEST_ARGS)

ifeq (go.mod,$(wildcard go.mod))
# If go.mod exists (as determined above), assume we're vendoring
vendor/modules.txt: go.mod
	go mod download
	go mod vendor
endif

CODEOWNERS: CODEOWNERS.in
	$(SCRIPTS_DIR)/gen-codeowners

# This removes all Submariner-provided images and all untagged images
# Use this to ensure you use current images
# Copied from Makefile to provide this everywhere (until we can share
# non-Dapper goals across projects)
prune-images:
	docker images | grep -E '(admiral|lighthouse|nettest|shipyard|submariner|<none>)' | while read image tag hash _; do \
	    if [ "$$tag" != "<none>" ]; then \
	        docker rmi $$image:$$tag; \
	    else \
	        docker rmi $$hash; \
	    fi \
	done
