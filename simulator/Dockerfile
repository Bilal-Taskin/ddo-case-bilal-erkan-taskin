# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/fitgiris-simulator ./cmd/simulator

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

COPY --from=builder /app/fitgiris-simulator .

USER appuser

ENTRYPOINT ["./fitgiris-simulator"]
