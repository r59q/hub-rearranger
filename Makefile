.PHONY: check test lint format-check runtime-check

RUNTIME_PYTHON ?= ops/private-runner/.venv/bin/python

check: format-check lint test runtime-check

runtime-check:
	$(RUNTIME_PYTHON) -m ruff format --check ops/private-runner
	$(RUNTIME_PYTHON) -m ruff check ops/private-runner
	$(RUNTIME_PYTHON) -m unittest discover -s ops/private-runner -v
	node --test ops/private-runner/test_authorize.mjs
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/*.yml

format-check:
	@test -z "$$(find services -type f -name '*.go' -exec gofmt -l {} +)" || \
		(echo "Go files need formatting:"; find services -type f -name '*.go' -exec gofmt -l {} +; exit 1)
	cd frontend && npm exec prettier -- --check .

lint:
	@for module in services/*; do (cd "$$module" && go vet ./...) || exit 1; done
	cd frontend && npm run check
	cd frontend && npm exec eslint -- .

test:
	@for module in services/*; do (cd "$$module" && go test ./...) || exit 1; done
	cd frontend && npm test
