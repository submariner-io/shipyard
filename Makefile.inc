, := ,
_using = $(subst $(,), ,$(using))

include $(SHIPYARD_DIR)/Makefile.images
include $(SHIPYARD_DIR)/Makefile.versions

# Process extra flags from the `using=a,b,c` optional flag

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

gitlint:
	gitlint --commits origin/master..HEAD

yamllint:
	yamllint .

release:
	$(SCRIPTS_DIR)/release.sh $(RELEASE_ARGS)

post-mortem:
	$(SCRIPTS_DIR)/post_mortem.sh

unit-test: vendor/modules.txt
	$(SCRIPTS_DIR)/unit_test.sh $(UNIT_TEST_ARGS)

validate: vendor/modules.txt
	$(SCRIPTS_DIR)/validate.sh $(VALIDATE_ARGS)

ifeq (go.mod,$(wildcard go.mod))
# If go.mod exists (as determined above), assume we're vendoring
vendor/modules.txt: go.mod
	go mod download
	go mod vendor
endif
