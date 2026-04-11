# CLAUDE.md

このファイルは、Claude Code (claude.ai/code) がこのリポジトリで作業する際のガイドです。

## プロジェクト概要

EXVS Analyzer は、EXVS2XB（機動戦士ガンダム エクストリームバーサス2 クロスブースト）の戦績分析Webアプリ。公式サイトから対戦データをスクレイピングし、GCSにCSVとして保存、Python分析を実行してMarkdownレポートを返す。

## ビルド・開発コマンド

```bash
# ビルド＆起動（初回・コード変更時）
make restart

# ビルドのみ
make build

# 起動のみ（ビルド済みの場合）
make run

# コンテナ停止
make stop

# Goテスト
make test

# ポート変更
PORT=3000 make run

# フロントエンド確認（サーバー不要）
# static/preview.html をローカルHTTPサーバーで開く
python3 -m http.server 8888 --directory static
# → http://localhost:8888/preview.html

# レポートプレビュー更新
python3 scripts/analyze.py /tmp/scores.csv
cp /tmp/report.md static/sample_report.md

# Pulumiコマンド（Docker経由）
PULUMI_CONFIG_PASSPHRASE=<passphrase> make pulumi-preview   # 変更プレビュー
PULUMI_CONFIG_PASSPHRASE=<passphrase> make pulumi-up        # 変更適用
```

http://localhost:8080 でアクセス可能。

**ローカル環境にはGo/Python/Pulumiはインストールされていない。** すべてDocker経由（Makefile）で実行する。Pulumi操作時は `infra/.envrc`（direnv）から `PULUMI_CONFIG_PASSPHRASE` が自動で読み込まれる。

CIでは `go vet`、`go build`、`py_compile` を実行。ラベル `skip-ci` でスキップ可能。

## アーキテクチャ

Go HTTPサーバーによる**非同期ジョブパイプライン**（最大同時実行数: 3）:

```
ブラウザ → POST /analyze → ジョブ作成（pending）
  → GCSから既存CSVをダウンロード
  → Collyで新規戦績をスクレイピング（状態: scraping）
  → CSVをマージし、data/ms_list.jsonからMS名・コストを補完
  → CSVをGCSにアップロード
  → scripts/analyze.py でCSVを分析（状態: analyzing）
  → Markdownレポートを返却（状態: done）
クライアントは GET /status/{id} でポーリング後、GET /result/{id} で結果取得
```

**主要エンドポイント:** `POST /analyze`, `GET /status/{id}`, `GET /result/{id}`, `GET /health`, `GET /`（静的UI）

## コード構成

- `cmd/server/main.go` — エントリポイント。`internal/server.StartServer()` に委譲
- `cmd/update-mslist/main.go` — MSリストをスクレイピングして `data/ms_list.json` を更新するCLI
- `internal/model/` — 型定義のみ（`PlayerScore`, `DatedScore`, `MSInfo`, `AverageScore`）
- `internal/mslist/` — MSリストの読み書き・マージ（`LoadMSList`, `SaveMSList`, `MergeMSList`, `BuildMSNameMap`, `FillMsNames`, `CheckUnknownMS`）
- `internal/scraper/` — Collyベースのスクレイパー（`scraper.go`）+ バンダイナムコID認証（`login.go`）
- `internal/pipeline/` — 分析パイプライン（`Job`型、ジョブストア、`Run`関数）
- `internal/server/` — HTTPハンドラ、レート制限
- `internal/storage/` — CSV読み書き（`csv_export.go`）+ GCSアップロード/ダウンロード（`cloud_storage.go`）
- `scripts/analyze.py` — Python分析: 目次、カテゴリ別アドバイス、勝率、与被ダメ比、固定相方検出、Markdownレポート生成
- `static/index.html` — SPA フロントエンド（ダークテーマ、レスポンシブ対応）
- `static/app.js` — フロントエンドJS（CSP対応で外部化。DOMPurify + marked.jsでレンダリング）
- `static/preview.html` — フロントエンド開発用プレビュー（gitignore対象）
- `data/ms_list.json` — MS画像URL→名前・コストのマッピング（コスト: 3000/2500/2000/1500）
- `infra/` — Pulumi IaC（Cloud Run、Artifact Registry、予算アラート等）

## GitHub Actions

- CI: `ci.yml`（PRのみ。Docker build, go vet, py_compile。ラベル `skip-ci` でスキップ）
- CD: `cd.yml`（mainマージ時 or 手動実行。Pulumi経由でCloud Runへデプロイ。ラベル `no-deploy` でスキップ）
- Infra CI: `infra-ci.yml`（infra/配下の変更時にPulumi preview）
- MSリスト更新: `update-mslist.yml`（毎日03:00-06:00 JST、ランダムスリープ。変更時にPR自動作成）
- **サードパーティアクションを追加・変更する際は、GitHubリポジトリのリリースページで最新メジャーバージョンを確認すること。** 古いバージョンを指定するとNode.js非推奨警告やエラーが発生する（過去に複数回発生）。

## PR運用ルール

- PRには基本的に `no-deploy` ラベルを付ける（デプロイはまとめて行う）
- Go/Docker以外の軽微な変更には `skip-ci` ラベルを付ける
- デプロイは `gh workflow run cd.yml` で手動実行、または `no-deploy` なしでPRをマージ
- **コード構成やディレクトリ構造に変更があった場合は、CLAUDE.mdの「コード構成」セクションとREADME.mdのプロジェクト構成も合わせて更新すること**

## 主要な技術情報

- **Go 1.26**、Webフレームワーク不使用（標準 `net/http`）
- **Python 3.11** で分析（pip依存なし）
- **Pulumi (TypeScript)** でインフラ管理
- GCSバケットは環境変数 `GCS_BUCKET` で指定、ユーザーキーは SHA256(email)[:8] の16進数
- Cloud Runデプロイ、`PORT` 環境変数（デフォルト 8080）

## Goコーディング規約

- **パッケージの責務を明確に分離する。** 1パッケージ1責務。型定義パッケージにI/Oやビジネスロジックを混ぜない
- **`cmd/`にはmain関数のみ。** ロジックは`internal/`に置く
- **`log.Fatal`はmain関数の初期化時のみ使用可。** リクエスト処理中は`return error`でハンドリングする
- **エラーは`fmt.Errorf("文脈: %w", err)`でラップして返す。** 呼び出し元でハンドリングできるようにする
- **未使用のエクスポート関数は削除する。** テストでしか使われない関数はエクスポートしない
- **循環依存を作らない。** 依存は`model` ← `mslist` / `scraper` / `storage` ← `pipeline` ← `server`の一方向
- **構造体のフィールド名はGoの命名規則に従う。** `Give_damage`のようなスネークケースは新規コードでは使わない（既存は後方互換のため維持）
- **テストは対象パッケージと同じディレクトリに置く。** `xxx_test.go`で`package xxx`を使う
- **`go vet`と`make build`がパスすることを確認してからコミットする**

## セキュリティルール

- **GCPプロジェクトID、バケット名、サービスアカウント等のインフラ識別子をコードやCLAUDE.mdにハードコードしない。** 環境変数またはGitHub Secrets/Variablesを使うこと。
- **IAM権限は最小権限の原則を徹底する。** プロジェクトレベルの広範なロール（例: `roles/storage.admin`）ではなく、バケット単位・リソース単位で必要最低限のロール（例: `roles/storage.objectUser`）を付与すること。
- 公開リポジトリのため、コミット履歴にも残ることを意識する。
- マルチステージDockerfile: `golang:1.26-alpine` でビルド、`python:3.11-alpine` で実行
- CSP: `script-src 'self'`（インラインスクリプト禁止）、`style-src 'self' 'unsafe-inline'`
