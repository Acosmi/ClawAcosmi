# Multi-stage build for UHMS Go API
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /uhms-api cmd/server/main.go

# Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /uhms-api /usr/local/bin/uhms-api

EXPOSE 8080
ENV UHMS_PORT=8080
CMD ["uhms-api"]
