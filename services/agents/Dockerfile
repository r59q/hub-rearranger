FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /agents-service ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /agents-service /usr/local/bin/agents-service
USER app
ENV AGENTS_ADDR=:8082
EXPOSE 8082
ENTRYPOINT ["agents-service"]
