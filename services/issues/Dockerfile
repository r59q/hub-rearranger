FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /issues-service ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /issues-service /usr/local/bin/issues-service
USER app
ENV ISSUES_ADDR=:8081
EXPOSE 8081
ENTRYPOINT ["issues-service"]
