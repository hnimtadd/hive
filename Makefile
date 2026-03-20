.PHONY: generate
generate:
	@echo "---- Generating protobuf stuffs ---"
	buf generate
	go mod tidy

.PHONY: clean
clean:
	rm -rf gen/*
