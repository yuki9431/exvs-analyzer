# EXVS2IB 戦績分析ツール (exvs-analyzer)

機動戦士ガンダム EXTREME VS.2 INFINITE BOOST の戦績をスクレイピングし、分析レポートを生成するWebアプリケーション。

## プロジェクト構成

```
.
├── cmd/
│   ├── server/
│   │   └── main.go              # エントリポイント（サーバー起動のみ）
│   └── update-mslist/
│       └── main.go              # MSリスト更新CLI
├── internal/
│   ├── model/
│   │   └── types.go             # 型定義（PlayerScore, MSInfo等）
│   ├── scraper/
│   │   ├── scraper.go           # スクレイピング処理
│   │   └── login.go             # バンダイナムコIDログイン
│   ├── server/
│   │   ├── server.go            # HTTPサーバー・API
│   │   └── ratelimit.go         # IP単位のレート制限
│   └── storage/
│       ├── csv_export.go        # CSV読み書き
│       └── cloud_storage.go     # Cloud Storage連携
├── scripts/
│   └── analyze.py               # Python分析スクリプト
├── static/
│   ├── index.html               # フロントエンド
│   ├── marked.min.js            # Markdownパーサー（self-hosting）
│   └── purify.min.js            # DOMPurify（XSSサニタイズ）
├── data/
│   └── ms_list.json             # 機体名マッピング（261件）
├── .github/
│   └── workflows/
│       ├── ci.yml               # CI（Docker build, Go vet, Python構文チェック）
│       ├── cd.yml               # CD（Cloud Runへ自動デプロイ）
│       └── update-mslist.yml    # MSリスト自動更新（毎日実行）
├── Makefile                     # ビルド・起動コマンド
├── Dockerfile                   # マルチステージビルド
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

### ディレクトリの役割

| ディレクトリ | 説明 |
|-------------|------|
| `cmd/` | エントリポイント。main関数のみ |
| `internal/` | プライベートパッケージ。外部から参照不可 |
| `internal/model/` | データ型の定義 |
| `internal/scraper/` | スクレイピング・ログイン処理 |
| `internal/server/` | HTTPサーバー・APIエンドポイント・レート制限 |
| `internal/storage/` | CSV・Cloud Storageの読み書き |
| `scripts/` | Go以外のスクリプト（Python分析等） |
| `static/` | フロントエンドHTML/JS |
| `data/` | 静的データファイル |

## 使い方

```bash
# ビルド＆起動（初回・コード変更時）
make restart

# ビルドのみ
make build

# 起動のみ（ビルド済みの場合）
make run

# コンテナ停止
make stop
```

http://localhost:8080 にアクセスしてバンナムIDでログインすると分析レポートが表示されます。

ポートを変更したい場合は `PORT=3000 make run` のように指定できます。

## セキュリティ

- リクエストボディサイズ制限（1KB）・入力長制限
- メールアドレス形式バリデーション
- IP単位のレート制限（環境変数`RATE_LIMIT`で設定）
- DOMPurifyによるXSSサニタイズ
- セキュリティヘッダー（CSP, X-Content-Type-Options, X-Frame-Options, HSTS）
- エラーメッセージの内部情報漏洩防止
- JSライブラリのself-hosting（CDN依存排除）
- HTTPクライアントタイムアウト（30秒）
- メモリリーク防止（レートリミッター・ジョブの定期クリーンアップ）

## 技術スタック

- **バックエンド**: Go 1.26
- **分析**: Python 3.11
- **インフラ**: Cloud Run (GCP)
- **ストレージ**: Cloud Storage (GCP)
- **CI/CD**: GitHub Actions
- **コンテナ**: Docker（マルチステージビルド）

## Author

Dillen Hiroyuki ([@yuki9431](https://github.com/yuki9431))

## License

[Apache License 2.0](LICENSE)
