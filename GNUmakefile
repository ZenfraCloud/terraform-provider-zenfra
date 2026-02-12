# ABOUTME: Build and development targets for the Zenfra Terraform provider.
# ABOUTME: Provides build, install, test, acceptance test, lint, and format targets.

BINARY_NAME  := terraform-provider-zenfra
INSTALL_DIR  := ~/.terraform.d/plugins/registry.terraform.io/zenfra/zenfra/0.0.1/$(shell go env GOOS)_$(shell go env GOARCH)
GOFLAGS      := -trimpath

.PHONY: build install test testacc lint fmt clean

build:
	go build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/terraform-provider-zenfra

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/

test:
	go test -v -race -count=1 ./...

testacc:
	TF_ACC=1 go test -v -race -count=1 -timeout 120m ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

clean:
	rm -f $(BINARY_NAME)
