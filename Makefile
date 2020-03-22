k8s_version ?= 1.14.6
globalnet ?= false

TARGETS := $(shell ls scripts)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

shell:
	./.dapper -m bind -s

$(TARGETS): .dapper dapper-image
	./.dapper -m bind $@ --k8s_version $(k8s_version) --globalnet $(globalnet)

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
