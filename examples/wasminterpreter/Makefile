BIN_PATH=../../_output/bin/linux/amd64
WASM_MODULE_NAME=interpreter.wasm

.PHONY: build-wasm
build-wasm:
	rm -rf $(WASM_MODULE_NAME)
	GOOS=js GOARCH=wasm go build -o $(BIN_PATH)/$(WASM_MODULE_NAME) main.go
	# CGO_ENABLED=0 tinygo build -O $(BIN_PATH)/$(WASM_MODULE_NAME) -scheduler=none -tags=appengine -target=wasi main.go
