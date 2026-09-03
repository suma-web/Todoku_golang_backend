# AWS構成

Todokuの本番環境は、フロントエンドをAWS Amplify Hosting、APIをAmazon ECS、データベースをAmazon RDS for PostgreSQL、添付ファイルをAmazon S3で提供しています。

```mermaid
flowchart LR
    user[利用者のブラウザ]
    domain[独自ドメイン<br/>todoku-service.com]

    subgraph frontend[フロントエンド]
        amplify[AWS Amplify Hosting<br/>React / TypeScript]
    end

    subgraph backend[バックエンド]
        ingress[ECS公開HTTPSエンドポイント]
        ecs[Amazon ECS Service<br/>Go API x 2]
        ecr[Amazon ECR<br/>コンテナイメージ]
    end

    subgraph data[データ・認証情報]
        rds[(Amazon RDS<br/>PostgreSQL)]
        s3[(Amazon S3<br/>非公開添付バケット)]
        secrets[AWS Secrets Manager<br/>DB・セッション秘密情報]
    end

    subgraph operations[監視・権限]
        logs[Amazon CloudWatch<br/>ログ・デプロイアラーム]
        role[IAM ECSタスクロール]
    end

    user -->|HTTPS| domain
    domain --> amplify
    amplify -->|Cookie認証付きAPI通信| ingress
    ingress --> ecs
    ecr -->|イメージ取得| ecs
    ecs -->|SQL / TLS| rds
    ecs -->|認証後に取得・保存| s3
    secrets -->|実行時に注入| ecs
    ecs -->|アプリケーションログ| logs
    role -. S3オブジェクト権限 .-> ecs
    role -. Get / Put / Delete .-> s3
```

## セキュリティ上の境界

- APIはセッションCookieで利用者を認証し、Roleと所属情報に基づいて操作・閲覧を制御します。
- RDS接続にはTLSを使用し、認証情報とセッション秘密情報はSecrets ManagerからECSへ渡します。
- S3はパブリックアクセスを全面的に遮断しています。ブラウザからS3へ直接アクセスさせず、APIで認証・閲覧権限を確認してから配信します。
- ECSタスクロールは添付オブジェクトの取得・保存・削除に限定しています。
- ECSは2タスクで稼働し、CloudWatchアラームとサーキットブレーカーを伴うカナリアデプロイを使用します。

## デプロイ経路

```mermaid
flowchart LR
    github[GitHub main]
    amplifyBuild[Amplify Build]
    frontendProd[Amplify Hosting]
    docker[Docker Build<br/>linux/amd64]
    ecr[Amazon ECR]
    ecsDeploy[ECS Canary Deployment]
    backendProd[ECS Service]

    github --> amplifyBuild --> frontendProd
    github --> docker --> ecr --> ecsDeploy --> backendProd
```

添付ファイル用S3バケットとECSタスクロールは、[`deploy/aws/attachment-storage.yml`](../../deploy/aws/attachment-storage.yml)でCloudFormation管理しています。ECS、RDS、Amplifyを含む全環境のIaC化は今後の改善項目です。
