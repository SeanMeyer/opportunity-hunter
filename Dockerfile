FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o opportunity-hunter ./cmd/opportunity-hunter/

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/opportunity-hunter /usr/local/bin/
ENV DB_PATH=/data/opportunity-hunter.db
VOLUME ["/data"]
ENTRYPOINT ["opportunity-hunter"]
CMD ["run"]
