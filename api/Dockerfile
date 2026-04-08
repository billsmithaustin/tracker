FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod *.go ./
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o api .

FROM alpine:3.21
# ca-certificates is required for TLS connections to api.open-meteo.com
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/api .
EXPOSE 3000
CMD ["./api"]
