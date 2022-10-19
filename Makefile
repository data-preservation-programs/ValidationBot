SHELL=/usr/bin/env bash

all: deps build
## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build/.filecoin-install ;

build/.filecoin-install: $(FFI_PATH)
	@mkdir -p build
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build/.filecoin-install
CLEAN+=build/.filecoin-install

ffi-version-check:
	@[[ "$$(awk '/const Version/{print $$5}' extern/filecoin-ffi/version.go)" -eq 3 ]] || (echo "FFI version mismatch, update submodules"; exit 1)
BUILD_DEPS+=ffi-version-check

.PHONY: ffi-version-check

$(MODULES): build/.update-modules ;
# dummy file that marks the last time modules were updated
build/.update-modules:
	git submodule update --init --recursive
	@mkdir -p build
	touch $@

CLEAN+=build/.update-modules

.PHONY: deps
deps: $(BUILD_DEPS)

build:
	go build -o validation_bot cmd/validation_bot.go

run:
	go run cmd/validation_bot.go

clean:
	go clean
	rm -f validation_bot

fmt:
	gofumpt -w .

lint:
	golangci-lint run

test:
	go test -p 4 -v ./...


.PHONY: build run clean test
