# EXVS2IB 戦績分析ツール (exvs-analyzer)

機動戦士ガンダム EXTREME VS.2 INFINITE BOOST の戦績をスクレイピングし、分析レポートを生成するWebアプリケーション。

## プロジェクト構成

```
.
├── cmd/
│   └── server/
│       └── main.go              # エントリポイント（サーバー起動のみ）
├── internal/
│   ├── model/
│   │   └── types.go             # 型定義（PlayerScore, MSInfo等）
│   ├── scraper/
│   │   ├── scraper.go           # スクレイピング処理
│   │   └── login.go             # バンダイナムコIDログイン
│   ├── server/
│   │   └── server.go            # HTTPサーバー・API
│   └── storage/
│       ├── csv_export.go        # CSV読み書き
│       └── cloud_storage.go     # Cloud Storage連携
├── scripts/
│   ├── analyze.py               # Python分析スクリプト
│   ├── docker.sh                # Docker操作スクリプト
│   └── entrypoint.sh            # Docker CLIモード用
├── static/
│   └── index.html               # フロントエンド
├── data/
│   └── ms_list.json             # 機体名マッピング（261件）
├── .github/
│   └── workflows/
│       └── ci.yml               # CI（Docker build, Go vet, Python構文チェック）
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
| `internal/server/` | HTTPサーバー・APIエンドポイント |
| `internal/storage/` | CSV・Cloud Storageの読み書き |
| `scripts/` | Go以外のスクリプト（Python分析等） |
| `static/` | フロントエンドHTML/JS |
| `data/` | 静的データファイル |

## 使い方

```bash
# ビルド＆起動（初回・コード変更時）
./scripts/docker.sh restart

# ビルドのみ
./scripts/docker.sh build

# 起動のみ（ビルド済みの場合）
./scripts/docker.sh run
```

http://localhost:8080 にアクセスしてバンナムIDでログインすると分析レポートが表示されます。

ポートを変更したい場合は `PORT=3000 ./scripts/docker.sh run` のように指定できます。

## 技術スタック

- **バックエンド**: Go 1.26
- **分析**: Python 3.11
- **インフラ**: Cloud Run (GCP)
- **ストレージ**: Cloud Storage (GCP)
- **CI**: GitHub Actions
- **コンテナ**: Docker（マルチステージビルド）

## Author

Dillen Hiroyuki ([@yuki9431](https://github.com/yuki9431))

## License

[Apache License 2.0](LICENSE)
