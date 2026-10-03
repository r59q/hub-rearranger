FROM golang:1.25-alpine AS build

WORKDIR /source/services/agents
COPY services/agents/go.mod services/agents/go.sum ./
RUN go mod download
COPY services/agents/ ./
COPY .github/ /source/.github/
COPY ops/agent-intake/ /source/ops/agent-intake/
COPY ops/agent-profiles/ /source/ops/agent-profiles/
COPY ops/private-runner/ /source/ops/private-runner/
COPY docs/agent-workflows.md /source/docs/agent-workflows.md
RUN go run ./cmd/bootstrap --source /source > /bootstrap.json
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /agents-service ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache git
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /agents-service /usr/local/bin/agents-service
COPY --from=build /bootstrap.json /app/bootstrap.json
USER app
ENV AGENTS_ADDR=:8082
ENV AGENTS_BOOTSTRAP_BUNDLE=/app/bootstrap.json
EXPOSE 8082
ENTRYPOINT ["agents-service"]
