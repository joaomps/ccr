build:
	cd engine && go build -o ../ccr-engine ./cmd/ccr-engine

test:
	cd engine && go test ./...

vet:
	cd engine && go vet ./...

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 ccr-engine $(HOME)/.local/bin/ccr-engine

.PHONY: build test vet install
