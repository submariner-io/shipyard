, := ,
_using = $(subst $(,), ,$(using))

include $(SHIPYARD_DIR)/Makefile.images
include $(SHIPYARD_DIR)/Makefile.versions

# Process extra flags from the `using=a,b,c` optional flag

ifneq (,$(filter libreswan,$(_using)))
override DEPLOY_ARGS += --cable_driver libreswan
else ifneq (,$(filter strongswan,$(_using)))
override DEPLOY_ARGS += --cable_driver strongswan
else ifneq (,$(filter wireguard,$(_using)))
# Wireguard requires kernel module install on the host
override DEPLOY_ARGS += --cable_driver wireguard
endif

ifneq (,$(filter globalnet,$(_using)))
override CLUSTERS_ARGS += --globalnet
override DEPLOY_ARGS += --globalnet
override E2E_ARGS += --globalnet
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

# Shipyard provided targets

cleanup:
	$(SCRIPTS_DIR)/cleanup.sh

clusters:
	$(SCRIPTS_DIR)/clusters.sh $(CLUSTERS_ARGS)

deploy: clusters
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
	$(MAKE) -o images deploy SUBCTL=~/.local/bin/subctl DEV_VERSION=latest CUTTING_EDGE=latest VERSION=latest DEPLOY_ARGS="$(DEPLOY_ARGS) --image_tag=latest" using=$(using)

gitlint:
	gitlint --commits origin/master..HEAD

golangci-lint: vendor/modules.txt
	golangci-lint linters
	golangci-lint run --timeout 10m

lint: gitlint golangci-lint markdownlint yamllint

markdownlint:
	markdownlint markdownlint -c .markdownlint.yml -i vendor .

yamllint:
	yamllint .

release:
	$(SCRIPTS_DIR)/release.sh $(RELEASE_ARGS)

post-mortem:
	$(SCRIPTS_DIR)/post_mortem.sh

unit: vendor/modules.txt
	$(SCRIPTS_DIR)/unit_test.sh $(UNIT_TEST_ARGS)

# TODO: Remove once all consumers are migrated to "unit"
unit-test: vendor/modules.txt
	$(SCRIPTS_DIR)/unit_test.sh $(UNIT_TEST_ARGS)

ifeq (go.mod,$(wildcard go.mod))
# If go.mod exists (as determined above), assume we're vendoring
vendor/modules.txt: go.mod
	go mod download
	go mod vendor
endif

CODEOWNERS: CODEOWNERS.in
	$(SCRIPTS_DIR)/gen-codeowners
