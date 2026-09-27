.PHONY: check test lint format-check

check: format-check lint test

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
