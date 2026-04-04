FROM golang:1.19-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o scraper .

FROM python:3.11-alpine

WORKDIR /app

# Goバイナリをコピー
COPY --from=builder /app/scraper .

# Python分析スクリプトをコピー
COPY analyze.py .

# エントリポイント
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]
