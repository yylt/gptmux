ifneq "$(strip $(shell command -v go 2>/dev/null))" ""
	GOOS ?= $(shell go env GOOS)
	GOARCH ?= $(shell go env GOARCH)
else
	ifeq ($(GOOS),)
		# approximate GOOS for the platform if we don't have Go and GOOS isn't
		# set. We leave GOARCH unset, so that may need to be fixed.
		ifeq ($(OS),Windows_NT)
			GOOS = windows
		else
			UNAME_S := $(shell uname -s)
			ifeq ($(UNAME_S),Linux)
				GOOS = linux
			endif
			ifeq ($(UNAME_S),Darwin)
				GOOS = darwin
			endif
			ifeq ($(UNAME_S),FreeBSD)
				GOOS = freebsd
			endif
		endif
	else
		GOOS ?= $$GOOS
		GOARCH ?= $$GOARCH
	endif
endif

COMMIT = $(shell git rev-parse --short HEAD)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
GOVER=$(shell go version | cut -d ' ' -f 3)
REVERSION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
GIT_BRANCH=$(shell git branch|grep ^\*| cut -d ' ' -f 2)

PKG=github.com/yylt/chatmux
PACKAGE=github.com/yylt/chatmux/cmd

GOPATHS=$(shell echo ${GOPATH} | tr ":" "\n" | tr ";" "\n")

GO_GCFLAGS=$(shell				\
	set -- ${GOPATHS};			\
	echo "-gcflags=-trimpath=$${1}/src";	\
	)
GO_LDFLAGS=-ldflags '-s -w -X $(PKG)/version.Version=$(COMMIT) -X $(PKG)/version.Goversion=$(GOVER) -X $(PKG)/version.Dirty=$(GIT_DIRTY) -X $(PKG)/version.Branch=$(GIT_BRANCH)'

NAME=chatmux

BINDIR=bin

.PHONY: all binary
all: binary

binary:
	@echo "build Project: ${PKG} "
	@echo "bin: ${PACKAGE} "
	@echo "git commit: ${COMMIT}; goversion: ${GOVER}"
	@echo "git branch: ${GIT_BRANCH}; git dirty: ${GIT_DIRTY}"
	go env -w CGO_ENABLED="0"
	go env -w GOPROXY=https://goproxy.cn/
	go build -o ${BINDIR}/${NAME} ${GO_GCFLAGS} $(GO_LDFLAGS) ${PACKAGE}