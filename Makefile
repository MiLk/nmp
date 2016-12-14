.PHONY: install build release

NAME:=nmp
PACKAGE:=milk/nmp

install:
	go install github.com/$(PACKAGE)/cmd/$(NAME)

build:
	@TF_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

release:
	@TF_RELEASE=1 sh -c "'$(CURDIR)/scripts/build.sh'"
