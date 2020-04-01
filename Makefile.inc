k8s_version ?= 1.14.6
globalnet ?= false
deploytool ?= operator
registry_inmemory ?= true

SCRIPTS_DIR ?= /opt/shipyard/scripts

cleanup:
	$(SCRIPTS_DIR)/cleanup.sh

clusters:
	$(SCRIPTS_DIR)/clusters.sh --k8s_version $(k8s_version) --globalnet $(globalnet) --registry_inmemory $(registry_inmemory)

deploy: clusters
	$(SCRIPTS_DIR)/deploy.sh --globalnet $(globalnet) --deploytool $(deploytool)
