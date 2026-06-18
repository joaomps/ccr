build:
	cd engine && go build -o ../ccr-engine ./cmd/ccr-engine

test:
	cd engine && go test ./...

vet:
	cd engine && go vet ./...

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 ccr-engine $(HOME)/.local/bin/ccr-engine

# Model-free self-check of the eval scorers (no review run, CI-safe).
eval-test:
	@bash eval/test_score.sh

# Review-quality scorecard: review each planted-bug fixture and report recall.
# Best-effort -- needs `claude -p` to drive a full review (see eval/run.sh).
eval:
	@for d in eval/fixtures/*/; do bash eval/recall.sh "$$d" || true; done

.PHONY: build test vet install eval eval-test
