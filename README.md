# 学校内情報・コミュニケーション基盤 Backend

[![Backend CI](https://github.com/suma-web/Todoku_golang_backend/actions/workflows/ci.yml/badge.svg)](https://github.com/suma-web/Todoku_golang_backend/actions/workflows/ci.yml)

学校内に分散している連絡・質問・相談を一つに集約し、必要な情報へアクセスしやすくするためのWebアプリケーションのバックエンドAPIです。

[本番環境](https://todoku-service.com) | [フロントエンド](https://github.com/suma-web/Todoku_react_frontend) | [デモ動画](https://github.com/suma-web/Todoku_react_frontend/releases/tag/demo-v1.0) | [AWS構成図](docs/architecture/aws.md) | [ER図](docs/architecture/erd/README.md)

## 解決したい課題

- Classroom、メール、口頭など、情報経路が複数の場所に分散している
- 生徒が誰に質問すべきか分からない
- 担当教員が不在のときに質問・相談が止まってしまう
- 配信した連絡を対象者の誰が既読にしたか分からない

## 解決方法

- **所属別情報配信**：学年、クラス、部活動、委員会、部署単位で対象者へ連絡を配信
- **質問の担当部署ルーティング**：質問カテゴリを担当部署と紐付け、適切な窓口へ届ける
- **公開質問 / 個別相談**：内容に応じて公開範囲を選択
- **明示的な既読**：連絡詳細で「既読にする」を押した状態を記録
- **横断検索**：閲覧権限を考慮し、お知らせ、質問・回答、担当窓口をまとめて検索

## 主な機能

- メールアドレスとパスワード、セッションCookieを用いたログイン・ログアウト
- 生徒・教員・管理者のRoleベースアクセス制御
- 学年、クラス、部活動、委員会、部署の所属管理
- 所属を対象にした学校連絡の作成・配信
- 詳細画面の「既読にする」による明示的な既読登録と、連絡作成者・管理者による既読・未読ユーザーの確認
- 期限切れ連絡の通常ユーザー向け非表示、期限なしの古い連絡に対する注意表示
- 投稿者本人または管理者による学校連絡の削除
- 質問カテゴリと担当部署の管理
- 公開質問、個別相談、回答、解決状態の管理
- 権限を考慮した学校内横断検索
- PDF・JPEG・PNG・WebPの添付と認証付きダウンロード
- 管理者によるアカウント追加（ユーザー名、メールアドレス、初期パスワード、Role）
- 管理者によるユーザーRole・有効状態の管理

## Role

| Role | 主な権限 |
| --- | --- |
| `student` | 連絡の閲覧・確認、質問・相談、検索 |
| `teacher` | 生徒の機能に加え、連絡作成、確認状況の閲覧、質問への回答 |
| `admin` | 教員の機能に加え、アカウント発行、ユーザー、所属、質問カテゴリの管理 |

## 使用技術

- Go 1.25
- chi
- PostgreSQL 16
- bcrypt
- Docker / Docker Compose
- AWS ECS / ECR / RDS / S3 / Secrets Manager / CloudWatch

画面実装、操作方法、スクリーンショットは[フロントエンドリポジトリ](https://github.com/suma-web/Todoku_react_frontend)を参照してください。

## セットアップ

PostgreSQLとGo APIを起動します。

```bash
docker compose up --build
```

| サービス | URL・ポート |
| --- | --- |
| Go API | `http://localhost:8080` |
| ヘルスチェック | `http://localhost:8080/health` |
| PostgreSQL | `localhost:5432` |

起動時に`migrations/`内のSQLが順番に適用されます。

旧SNS機能のテーブル定義は、適用済みマイグレーションの履歴を変更して既存環境を壊さないため残しています。APIとアプリケーションコードからは利用されません。

### 開発用データベース設定

| 項目 | 値 |
| --- | --- |
| Host | ローカル：`localhost` / Docker内：`postgres` |
| Port | `5432` |
| Database | `todoku` |
| User | `todoku_user` |
| Password | `todoku_password` |

```text
postgres://todoku_user:todoku_password@localhost:5432/todoku?sslmode=disable
```

## 主なAPI

| 分類 | エンドポイント例 |
| --- | --- |
| 認証 | `POST /api/login`、`POST /api/logout`、`GET /api/me` |
| ユーザー管理 | `POST /api/admin/users`、`GET /api/admin/users`、`PATCH /api/admin/users/{id}` |
| 所属管理 | `GET /api/school-groups`、`POST /api/school-groups` |
| 学校連絡 | `POST /api/school-posts`、`GET /api/timeline`、`GET /api/school-posts/{id}`、`PATCH /api/school-posts/{id}`、`DELETE /api/school-posts/{id}` |
| 既読状況 | `POST /api/school-posts/{id}/read`、`GET /api/school-posts/{id}/status`、`GET /api/me/school-posts` |
| 質問・相談 | `GET /api/questions`、`POST /api/questions`、`POST /api/questions/{id}/answers` |
| 質問カテゴリ管理 | `POST /api/question-categories`、`PATCH /api/question-categories/{id}` |
| 横断検索 | `GET /api/search?q={検索語}` |
| 添付 | `POST /api/school-posts/{id}/attachments`、`POST /api/questions/{id}/attachments`、`POST /api/answers/{id}/attachments`、`GET /api/attachments/{id}/download` |

学校機能APIは認証を必要とし、管理機能・教員機能にはRoleによるアクセス制御があります。

添付は1件の連絡・質問・回答に最大5ファイル、1ファイル10MB、合計25MBまでです。保存先S3バケットは`ATTACHMENT_BUCKET`、リージョンは`AWS_REGION`で指定します。バケットは非公開とし、ファイルは認証・閲覧権限を確認するAPI経由で配信します。

公開signup APIは提供していません。アカウントは管理者が認証済みの`POST /api/admin/users`を通じて発行します。

## テスト

```bash
go vet ./...
go test ./... -race -coverprofile=coverage.out
```

2026年9月4日時点で25テスト、全体ステートメントカバレッジ11.2%、添付パッケージ40.8%です。認証Cookie、Role・閲覧権限、private質問、所属別連絡、添付の形式・件数・容量、認証前のストレージアクセス防止を重点的に検証しています。

テスト対象、期待するリスク、実行結果の詳細は[テスト方針と結果](docs/TESTING.md)を参照してください。CIではrace detectorとカバレッジ下限10%を毎回検証します。

## 負荷試験・性能改善

1校平均約645ユーザー（生徒・教職員）を想定し、k6で`Login → ユーザー情報取得 → Timeline閲覧 → Logout`の実利用フローを再現しました。通常利用と緊急一斉アクセスを分けて計測し、CloudWatch / Container InsightsのECS・ALB・RDSメトリクスと照合しています。

| シナリオ | 最大VU | リクエスト数 | HTTP失敗率 | checks | p95 | ECSタスク数 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 通常利用 | 645 | 122,989 | 約0.007%（8件） | 99.99% | 989ms | 2 → 6 |
| 緊急一斉アクセス | 645 | 35,629 | 42.59%（15,175件） | 57.40% | 7.26秒 | 2 → 4 |

緊急時は19,799 iterationsが完了し、116 iterationsが中断しました。エンドポイント別p95はLogin 7.18秒、ユーザー情報取得6.04秒、Timeline 15.53秒、Logout 381.27msです。RDSとメモリには比較的余力がある一方でECS CPUが高負荷となる傾向があり、ECS CPU、Auto Scalingの追従時間、Login処理をボトルネック候補として切り分けています。

k6のCookieリセットに起因する誤った401と、Loginを過剰に実行する初期シナリオも検証中に発見して修正しました。通常負荷への対応は確認できましたが、全利用者の一斉アクセスは未達の性能課題として記録し、**計測 → 仮説 → 原因切り分け → 設定変更 → 再計測**のサイクルで改善を進めています。

## データモデル

現行スキーマのER図、編集可能なソース、再生成手順は[ER図ドキュメント](docs/architecture/erd/README.md)を参照してください。

## AWS添付ストレージ

S3バケットとECSタスクロールは`deploy/aws/attachment-storage.yml`で定義しています。バケットは公開アクセスをすべて拒否し、暗号化とバージョニングを有効にします。

```bash
aws cloudformation deploy \
  --stack-name todoku-attachment-storage \
  --template-file deploy/aws/attachment-storage.yml \
  --parameter-overrides BucketName=todoku-attachments-ACCOUNT-REGION \
  --capabilities CAPABILITY_NAMED_IAM
```

## デモデータ

管理者によるアカウント発行、所属配信、質問ルーティング、private相談、既読、横断検索を一連で確認できる開発用seedを用意しています。通常起動では自動投入されません。

```bash
docker compose up -d postgres
docker compose run --rm -e DEMO_SEED=true backend go run ./cmd/seed
```

アカウント情報、作成データ、期待するアクセス範囲、操作シナリオは[デモデータと確認手順](docs/DEMO.md)を参照してください。

## 学校機能の責務分離

`schooladmin`、`schoolgroup`、`schoolpost`、`question`、`search`は、次の依存方向で構成しています。

```text
Handler → Service → Repository → PostgreSQL
```

| 層 | 責務 |
| --- | --- |
| Handler | HTTP入力の解析、認証コンテキストの取得、ステータスコードとJSONレスポンス |
| Service | 入力値の正規化・検証、権限判定、処理手順などの業務ルール |
| Repository | SQL、トランザクション、検索結果のマッピング |
| Models | APIと各層で共有するデータ構造 |

ServiceはRepositoryインターフェースへ依存するため、PostgreSQLを起動せず単体テストできます。

## 主なディレクトリ構成

```text
.
├── cmd/api/main.go          # APIサーバーとルーティング
├── cmd/seed/main.go         # 明示実行する開発用デモデータ
├── internal/
│   ├── auth/                # 認証・Role認可
│   ├── database/            # PostgreSQL接続とマイグレーション
│   ├── schooladmin/         # 管理者向けユーザー管理
│   ├── schoolgroup/         # 所属管理
│   ├── schoolpost/          # 学校連絡・既読
│   ├── question/            # 質問・相談
│   ├── search/              # 権限を考慮した横断検索
│   └── user/                # ユーザー・認証
├── migrations/              # データベース定義
├── docs/DEMO.md             # デモアカウントと確認手順
├── docs/architecture/aws.md # AWS構成図
├── docs/architecture/erd/   # ER図のソース・SVG・PDF
├── scripts/generate-erd.sh  # ER図の再生成
├── Dockerfile
└── compose.yml
```
