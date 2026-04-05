# CLAUDE.md

このファイルは、Claude Code (claude.ai/code) がこのリポジトリで作業する際のガイドです。

## プロジェクト概要

EXVS Analyzer は、EXVS2XB（機動戦士ガンダム エクストリームバーサス2 クロスブースト）の戦績分析Webアプリ。公式サイトから対戦データをスクレイピングし、GCSにCSVとして保存、Python分析を実行してMarkdownレポートを返す。

## ビルド・開発コマンド

```bash
# ビルド＆起動（初回・コード変更時）
./scripts/docker.sh restart

# ビルドのみ
./scripts/docker.sh build

# 起動のみ（ビルド済みの場合）
./scripts/docker.sh run

# ポート変更
PORT=3000 ./scripts/docker.sh run
```

http://localhost:8080 でアクセス可能。

GoテストおよびMakefileは存在しない。CIでは `go vet`、`go build`、`py_compile` のみ実行。

## アーキテクチャ

Go HTTPサーバーによる**非同期ジョブパイプライン**（最大同時実行数: 3）:

```
ブラウザ → POST /analyze → ジョブ作成（pending）
  → GCSから既存CSVをダウンロード
  → Collyで新規戦績をスクレイピング（状態: scraping）
  → CSVをマージし、data/ms_list.jsonからMS名を補完
  → CSVをGCSにアップロード
  → scripts/analyze.py でCSVを分析（状態: analyzing）
  → Markdownレポートを返却（状態: done）
クライアントは GET /status/{id} でポーリング後、GET /result/{id} で結果取得
```

**主要エンドポイント:** `POST /analyze`, `GET /status/{id}`, `GET /result/{id}`, `GET /health`, `GET /`（静的UI）

## コード構成

- `cmd/server/main.go` — エントリポイント。`internal/server.StartServer()` に委譲
- `internal/server/` — HTTPハンドラ、ジョブキュー、パイプライン制御
- `internal/scraper/` — Collyベースのスクレイパー（`scraper.go`）+ バンダイナムコID OAuth認証（`login.go`）
- `internal/model/` — データ型（`PlayerScore`, `DatedScore`, `MSInfo`）とMS名マッピング
- `internal/storage/` — CSV読み書き（`csv_export.go`）+ GCSアップロード/ダウンロード（`cloud_storage.go`）
- `scripts/analyze.py` — Python分析: シーズン分類、勝率、ダメージ効率、固定相方検出、Markdownレポート生成
- `static/index.html` — SPA フロントエンド（素のJS、ダークテーマ、marked.jsでレンダリング）
- `data/ms_list.json` — MS画像URL→名前のマッピング（261件）

## 主要な技術情報

- **Go 1.26**、Webフレームワーク不使用（標準 `net/http`）
- **Python 3.11** で分析（pip依存なし）
- GCSバケット: `exvs2ib-analyzer-data`、ユーザーキーは SHA256(email)[:8] の16進数
- Cloud Runデプロイ、`PORT` 環境変数（デフォルト 8080）
- マルチステージDockerfile: `golang:1.26-alpine` でビルド、`python:3.11-alpine` で実行
