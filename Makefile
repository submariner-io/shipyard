k8s_version ?= 1.14.6
globalnet ?= false
deploytool ?= operator

TARGETS := $(shell ls -p scripts | grep -v /)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

shell:
	./.dapper -m bind -s

# Deployment needs clusters installed
deploy: .dapper dapper-image clusters

$(TARGETS): .dapper dapper-image
	./.dapper -m bind $@ --k8s_version $(k8s_version) --globalnet $(globalnet) --deploytool $(deploytool)

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
