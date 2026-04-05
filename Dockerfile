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

# Python分析スクリプトとMSリストをコピー
COPY analyze.py .
COPY ms_list.json .

# フロントエンド
COPY static/ static/

# CLIモード用エントリポイント
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Cloud Run はPORT環境変数を設定する
ENV PORT=8080
EXPOSE 8080

# デフォルトはサーバーモード
CMD ["./scraper", "serve"]
