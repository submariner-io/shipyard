
TARGETS := $(shell ls scripts | grep -v dapper-image)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

dapper-image: .dapper
	./.dapper -m bind dapper-image

shell:
	./.dapper -m bind -s

$(TARGETS): .dapper dapper-image
	./.dapper -m bind $@

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
