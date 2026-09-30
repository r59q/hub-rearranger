.PHONY: check test lint format-check runtime-check profiles-check generate-agents-contract agents-contract-check

RUNTIME_PYTHON ?= ops/private-runner/.venv/bin/python
PROFILE_PYTHON ?= $(RUNTIME_PYTHON)
OAPI_CODEGEN_VERSION := v2.8.0

check: format-check lint test runtime-check profiles-check agents-contract-check

generate-agents-contract:
	cd services/agents && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config api/codegen.yaml -o internal/api/contract/agents.gen.go api/openapi.yaml
	cd frontend && npm run generate:agents-api

agents-contract-check:
	@set -eu; task_tmp=$$(mktemp -d /tmp/hub-agents-contract.XXXXXX); \
	trap 'rm -f "$$task_tmp/agents.gen.go" "$$task_tmp/agents-contract.gen.ts"; rmdir "$$task_tmp"' EXIT; \
	(cd services/agents && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config api/codegen.yaml -o "$$task_tmp/agents.gen.go" api/openapi.yaml); \
	cmp services/agents/internal/api/contract/agents.gen.go "$$task_tmp/agents.gen.go" || { echo 'Agents Go contract is stale; run make generate-agents-contract.'; exit 1; }; \
	(cd frontend && npm exec openapi-typescript -- ../services/agents/api/openapi.yaml --enum-values -o "$$task_tmp/agents-contract.gen.ts" && npm exec prettier -- --config .prettierrc --write "$$task_tmp/agents-contract.gen.ts"); \
	cmp frontend/src/lib/server/agents-contract.gen.ts "$$task_tmp/agents-contract.gen.ts" || { echo 'Agents frontend contract is stale; run make generate-agents-contract.'; exit 1; }
	cd frontend && npm exec prettier -- --config .prettierrc --check ../services/agents/api/*.yaml ../compose.yaml ../.github/workflows/application-checks.yml

profiles-check:
	$(PROFILE_PYTHON) -m ruff format --check --config ops/private-runner/pyproject.toml ops/agent-profiles
	$(PROFILE_PYTHON) -m ruff check --config ops/private-runner/pyproject.toml ops/agent-profiles
	$(PROFILE_PYTHON) -m unittest discover -s ops/agent-profiles -v
	$(PROFILE_PYTHON) ops/agent-profiles/validate.py
	cd frontend && npm exec prettier -- --check ../ops/agent-profiles/schema.v1.json ../.github/agent-profiles.yml

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
