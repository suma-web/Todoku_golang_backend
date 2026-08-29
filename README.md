# 学校内情報・コミュニケーション基盤 Backend

学校内に分散している連絡・質問・相談を一つに集約し、必要な情報へアクセスしやすくするためのWebアプリケーションのバックエンドAPIです。

## 開発の背景

最初はTwitterクローンとして技術学習目的で開発していました。しかし、自身の教育現場での経験から「情報が複数の場所に分散している」「質問したいときに適切な教員が分からない」という課題に着目し、既存サービスを調査したうえで、学校内情報基盤として要件を再設計しました。

Twitterクローンとして実装した認証、投稿、コメント、通知などの技術要素を活かしながら、学校内での情報共有とコミュニケーションに必要な機能を追加しています。学校用途に不要なフォロー、いいね、リツイート、通常DMは削除しています。

## 解決したい課題

- Classroom、メール、口頭など、情報経路が複数の場所に分散している
- 生徒が誰に質問すべきか分からない
- 担当教員が不在のときに質問・相談が止まってしまう
- 配信した連絡が対象者に読まれ、確認されたか分からない

## 解決方法

- **所属別情報配信**：学年、クラス、部活動、委員会、部署単位で対象者へ連絡を配信
- **質問の担当部署ルーティング**：質問カテゴリを担当部署と紐付け、適切な窓口へ届ける
- **公開質問 / 個別相談**：内容に応じて公開範囲を選択
- **既読・確認**：閲覧状況と明示的な確認状況を記録
- **横断検索**：閲覧権限を考慮し、お知らせ、質問・回答、担当窓口をまとめて検索

## 主な機能

- セッションCookieを用いたログイン・ログアウト
- 生徒・教員・管理者のRoleベースアクセス制御
- 学年、クラス、部活動、委員会、部署の所属管理
- 所属を対象にした学校連絡の作成・配信
- 連絡の既読・確認状況、未確認者の取得
- 質問カテゴリと担当部署の管理
- 公開質問、個別相談、回答、解決状態の管理
- 権限を考慮した学校内横断検索
- 管理者によるアカウント追加（ユーザー名、メールアドレス、初期パスワード、Role）
- 管理者によるユーザーRole・有効状態の管理
- 投稿、コメント、コメント通知、ブックマーク

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
| Database | `twitter` |
| User | `twitter` |
| Password | `twitter_password` |

```text
postgres://twitter:twitter_password@localhost:5432/twitter?sslmode=disable
```

## 主なAPI

| 分類 | エンドポイント例 |
| --- | --- |
| 認証 | `POST /api/login`、`POST /api/logout`、`GET /api/me` |
| ユーザー管理 | `POST /api/admin/users`、`GET /api/admin/users`、`PATCH /api/admin/users/{id}` |
| 所属管理 | `GET /api/school-groups`、`POST /api/school-groups` |
| 学校連絡 | `POST /api/school-posts`、`GET /api/timeline`、`GET /api/school-posts/{id}` |
| 既読・確認 | `POST /api/school-posts/{id}/read`、`POST /api/school-posts/{id}/confirm`、`GET /api/school-posts/{id}/status` |
| 質問・相談 | `GET /api/questions`、`POST /api/questions`、`POST /api/questions/{id}/answers` |
| 横断検索 | `GET /api/search?q={検索語}` |
| 投稿・コメント | `GET /api/posts`、`POST /api/posts`、`POST /api/posts/{id}/comments` |
| 通知・ブックマーク | `GET /api/notifications`、`GET /api/bookmarks`、`POST /api/posts/{id}/bookmarks` |

学校機能APIは認証を必要とし、管理機能・教員機能にはRoleによるアクセス制御があります。

公開signup APIは提供していません。アカウントは管理者が認証済みの`POST /api/admin/users`を通じて発行します。

## テスト

```bash
go vet ./...
go test ./...
```

## 学校機能の責務分離

`question`、`schoolpost`、`search`は、次の依存方向で構成しています。

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
├── internal/
│   ├── auth/                # 認証・Role認可
│   ├── comment/             # 投稿へのコメント
│   ├── database/            # PostgreSQL接続とマイグレーション
│   ├── notification/        # コメント通知
│   ├── post/                # 投稿・ブックマーク
│   ├── schooladmin/         # 管理者向けユーザー管理
│   ├── schoolgroup/         # 所属管理
│   ├── schoolpost/          # 学校連絡・既読・確認
│   ├── question/            # 質問・相談
│   ├── search/              # 権限を考慮した横断検索
│   └── user/                # ユーザー・認証
├── migrations/              # データベース定義
├── Dockerfile
└── compose.yml
```
