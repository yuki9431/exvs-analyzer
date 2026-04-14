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
│   │   └── types.go             # 型定義のみ（PlayerScore, MSInfo等）
│   ├── mslist/
│   │   └── mslist.go            # MSリストの読み書き・マージ
│   ├── scraper/
│   │   ├── scraper.go           # スクレイピング処理
│   │   └── login.go             # バンダイナムコIDログイン
│   ├── pipeline/
│   │   └── pipeline.go          # 分析パイプライン（Job管理・実行）
│   ├── server/
│   │   ├── server.go            # HTTPサーバー・API
│   │   └── ratelimit.go         # IPベースレート制限
│   └── storage/
│       ├── csv_export.go        # CSV読み書き
│       └── cloud_storage.go     # Cloud Storage連携
├── scripts/
│   └── analyze.py               # Python分析スクリプト
├── static/
│   ├── index.html               # フロントエンド
│   ├── app.js                   # フロントエンドJS（CSP対応で外部化）
│   └── htm-preact-standalone.js # htm + Preactライブラリ
├── data/
│   └── ms_list.json             # 機体名・コストマッピング
├── infra/
│   ├── index.ts                 # Pulumiエントリポイント
│   ├── apis.ts                  # Google API有効化
│   ├── artifact-registry.ts     # Artifact Registry定義
│   ├── cloudrun.ts              # Cloud Run定義
│   ├── domain.ts                # カスタムドメイン・DNS定義
│   ├── storage.ts               # Cloud Storageバケット定義
│   ├── iam.ts                   # サービスアカウント・Workload Identity
│   └── budget.ts                # 予算アラート定義
├── .github/
│   └── workflows/
│       ├── ci.yml               # CI（Docker build, Go vet, Python構文チェック）
│       ├── cd.yml               # CD（Pulumi経由でCloud Runデプロイ）
│       ├── infra-ci.yml         # インフラCI（Pulumi preview）
│       └── update-mslist.yml    # MSリスト自動更新
├── Makefile                     # ビルド・起動・インフラコマンド
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
| `internal/model/` | データ型の定義のみ |
| `internal/mslist/` | MSリストの読み書き・マージ |
| `internal/scraper/` | スクレイピング・ログイン処理 |
| `internal/pipeline/` | 分析パイプライン（ジョブ管理・実行） |
| `internal/server/` | HTTPハンドラ・レート制限 |
| `internal/storage/` | CSV・Cloud Storageの読み書き |
| `scripts/` | Go以外のスクリプト（Python分析等） |
| `static/` | フロントエンドHTML/JS/CSS |
| `data/` | 静的データファイル（MSリスト等） |
| `infra/` | Pulumi IaC（GCPリソース管理） |

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

# Goテスト
make test
```

http://localhost:8080 にアクセスしてバンナムIDでログインすると分析レポートが表示されます。

ポートを変更したい場合は `PORT=3000 make run` のように指定できます。

## 分析機能

- 全体勝率・与被ダメ比・K/D比
- 機体別分析（基本データ、敵機体との相性、相方機体との相性）
- 固定相方分析（連続10戦以上）
- 被撃墜数と勝率の関係
- 時間帯別・曜日別の勝率
- 日別勝率推移
- シーズン別分析
- 総合アドバイス（カテゴリ別: 耐久管理、機体、時間帯、相方、メンタル）
- SNS共有機能（X, Bluesky, LINE）

## 技術スタック

- **バックエンド**: Go 1.26（標準 `net/http`）
- **分析**: Python 3.11
- **インフラ**: Cloud Run (GCP)、Pulumi (TypeScript) でIaC管理
- **ストレージ**: Cloud Storage (GCP)
- **CI/CD**: GitHub Actions（ラベルでCI/CDを制御）
- **コンテナ**: Docker（マルチステージビルド）
- **フロントエンド**: htm/Preact（ダークテーマ、レスポンシブ対応）

## Author

Dillen Hiroyuki ([@yuki9431](https://github.com/yuki9431))

## License

[Apache License 2.0](LICENSE)
