# ABOUTME: Build and development targets for the Zenfra Terraform provider.
# ABOUTME: Provides build, install, test, acceptance test, lint, and format targets.

BINARY_NAME  := terraform-provider-zenfra
INSTALL_DIR  := ~/.terraform.d/plugins/registry.terraform.io/zenfra/zenfra/0.0.1/$(shell go env GOOS)_$(shell go env GOARCH)
GOFLAGS      := -trimpath

.PHONY: build install test testacc lint fmt docs clean

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

docs: build
	@TMPDIR=$$(mktemp -d) && \
	echo 'provider_installation { dev_overrides { "registry.terraform.io/zenfra/zenfra" = "$(CURDIR)" } direct {} }' > "$$TMPDIR/.terraformrc" && \
	echo 'terraform { required_providers { zenfra = { source = "registry.terraform.io/zenfra/zenfra" } } } provider "zenfra" { api_token = "dummy" }' > "$$TMPDIR/main.tf" && \
	TF_CLI_CONFIG_FILE="$$TMPDIR/.terraformrc" terraform -chdir="$$TMPDIR" providers schema -json > providers-schema.json && \
	rm -rf "$$TMPDIR" && \
	python3 -c "import json; f=open('providers-schema.json'); d=json.load(f); f.close(); s=d['provider_schemas']; k=list(s.keys())[0]; s['zenfra']=s.pop(k) if k!='zenfra' else s[k]; f=open('providers-schema.json','w'); json.dump(d,f); f.close()" && \
	cd tools && go generate ./...

clean:
	rm -f $(BINARY_NAME) providers-schema.json
