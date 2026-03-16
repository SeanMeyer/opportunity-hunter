FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-X main.version=${VERSION}" -o /opportunity-hunter ./cmd/opportunity-hunter/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /data
COPY --from=builder /opportunity-hunter /usr/local/bin/opportunity-hunter
ENV DB_PATH=/data/opportunity-hunter.db
VOLUME ["/data"]
ENTRYPOINT ["opportunity-hunter"]
CMD ["run"]
