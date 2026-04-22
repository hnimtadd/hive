.PHONY: generate
generate:
	@echo "---- Generating protobuf stuffs ---"
	buf generate
	go mod tidy

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -rf gen/*

.PHONY: dev
dev:
	nix develop -c $$SHELL
