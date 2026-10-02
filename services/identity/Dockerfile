FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /identity-service ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /identity-service /usr/local/bin/identity-service
RUN mkdir -p /app/data && chown app:app /app/data
USER app
ENV IDENTITY_ADDR=:8083
EXPOSE 8083
ENTRYPOINT ["identity-service"]
