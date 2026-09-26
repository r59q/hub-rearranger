FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /repositories-service ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /repositories-service /usr/local/bin/repositories-service
RUN mkdir -p /app/data && chown app:app /app/data
USER app
ENV REPOSITORIES_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["repositories-service"]
