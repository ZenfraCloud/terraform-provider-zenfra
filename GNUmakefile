# ABOUTME: Build and development targets for the Zenfra Terraform provider.
# ABOUTME: Provides build, install, test, acceptance test, lint, and format targets.

BINARY_NAME  := terraform-provider-zenfra
INSTALL_DIR  := ~/.terraform.d/plugins/registry.terraform.io/ZenfraCloud/zenfra/0.0.1/$(shell go env GOOS)_$(shell go env GOARCH)
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
	@DOCSTMP=$$(mktemp -d) && \
	printf 'provider_installation {\n  dev_overrides {\n    "registry.terraform.io/ZenfraCloud/zenfra" = "$(CURDIR)"\n  }\n  direct {}\n}\n' > "$$DOCSTMP/.terraformrc" && \
	printf 'terraform {\n  required_providers {\n    zenfra = {\n      source = "registry.terraform.io/ZenfraCloud/zenfra"\n    }\n  }\n}\nprovider "zenfra" {\n  api_token = "dummy"\n}\n' > "$$DOCSTMP/main.tf" && \
	TF_CLI_CONFIG_FILE="$$DOCSTMP/.terraformrc" terraform -chdir="$$DOCSTMP" providers schema -json > providers-schema.json && \
	rm -rf "$$DOCSTMP" && \
	python3 -c "import json; f=open('providers-schema.json'); d=json.load(f); f.close(); s=d['provider_schemas']; k=list(s.keys())[0]; s['zenfra']=s.pop(k) if k!='zenfra' else s[k]; f=open('providers-schema.json','w'); json.dump(d,f); f.close()" && \
	cd tools && go generate ./...

clean:
	rm -f $(BINARY_NAME) providers-schema.json
