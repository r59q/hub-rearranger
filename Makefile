.PHONY: check test lint format-check

check: format-check lint test

format-check:
	@test -z "$$(find services/repositories -type f -name '*.go' -exec gofmt -l {} +)" || \
		(echo "Go files need formatting:"; find services/repositories -type f -name '*.go' -exec gofmt -l {} +; exit 1)
	cd frontend && npm exec prettier -- --check .

lint:
	cd services/repositories && go vet ./...
	cd frontend && npm run check
	cd frontend && npm exec eslint -- .

test:
	cd services/repositories && go test ./...
	cd frontend && npm test
