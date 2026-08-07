FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/app ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/tgcheck ./cmd/tgcheck

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
RUN mkdir -p /app/data /app/output

COPY --from=builder /bin/app /app/app
COPY --from=builder /bin/tgcheck /app/tgcheck

ENTRYPOINT ["/app/app"]
