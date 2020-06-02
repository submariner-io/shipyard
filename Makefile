ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

CLUSTER_SETTINGS_FLAG = --cluster_settings $(DAPPER_SOURCE)/scripts/cluster_settings
override CLUSTERS_ARGS += $(CLUSTER_SETTINGS_FLAG)
override DEPLOY_ARGS += $(CLUSTER_SETTINGS_FLAG)
override E2E_ARGS += $(CLUSTER_SETTINGS_FLAG) cluster1 cluster2

include $(SHIPYARD_DIR)/Makefile.inc

TARGETS := $(shell ls -p scripts | grep -v -e /)

# Add any project-specific arguments here
$(TARGETS):
	./scripts/$@

.PHONY: $(TARGETS)

# Project-specific targets go here
deploy: nettest

nettest:
	$(SCRIPTS_DIR)/build_image.sh -i nettest -f package/Dockerfile.nettest

e2e validate: vendor/modules.txt

else

# Not running in Dapper

# Shipyard-specific starts
clusters deploy e2e nettest post-mortem release validate: dapper-image

dapper-image: export SCRIPTS_DIR=./scripts/shared

dapper-image:
	$(SCRIPTS_DIR)/build_image.sh -i shipyard-dapper-base -f package/Dockerfile.dapper-base $(dapper_image_flags)

.DEFAULT_GOAL := validate
# Shipyard-specific ends

include Makefile.dapper

endif

# Disable rebuilding Makefile
Makefile Makefile.dapper Makefile.inc: ;
