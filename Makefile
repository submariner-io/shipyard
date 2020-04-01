include Makefile.inc

TARGETS := $(shell ls -p scripts | grep -v -e /)

$(TARGETS): .dapper dapper-image
	./.dapper -m bind $@

# We need the latest image for these targets in shipyard
clusters: dapper-image
deploy: dapper-image

.PHONY: $(TARGETS)
