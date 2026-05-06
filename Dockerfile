FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ internal/
COPY cmd/ cmd/
RUN go build -o server ./cmd/server

FROM python:3.11-alpine

WORKDIR /app

# Goバイナリをコピー
COPY --from=builder /app/server .

# Python分析スクリプト
COPY scripts/analyze.py scripts/

# データファイル
COPY data/ms_list.json data/
COPY data/grade_list.json data/

# フロントエンド
COPY static/ static/

# Cloud Run はPORT環境変数を設定する
ENV PORT=8080
EXPOSE 8080

CMD ["./server"]
