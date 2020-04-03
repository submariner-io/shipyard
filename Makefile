ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

include $(SHIPYARD_DIR)/Makefile.inc

TARGETS := $(shell ls -p scripts | grep -v -e /)

# Add any project-specific arguments here
$(TARGETS):
	./scripts/$@

.PHONY: $(TARGETS)

# Project-specific targets go here

else

# Not running in Dapper

# Shipyard-specific starts
# Ensure we rebuild the base image if contents have changed
clusters deploy validate: $(shell scripts/base-needs-rebuild Makefile.inc scripts/shared)

dapper-image:
	SCRIPTS_DIR=./scripts/shared ./scripts/dapper-image
# Shipyard-specific ends

include Makefile.dapper

endif

# Disable rebuilding Makefile
Makefile Makefile.dapper Makefile.inc: ;
