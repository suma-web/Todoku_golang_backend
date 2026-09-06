# AWS構成・デプロイ経路

Todokuの本番環境は、フロントエンドをAWS Amplify Hosting、Go APIをAmazon ECS on AWS Fargate、データベースをAmazon RDS for PostgreSQL、添付ファイルをAmazon S3で提供しています。図は2026年9月6日確認のAWS構成図・デプロイ経路図です。

## AWS構成図

![Todoku AWS構成図](aws-architecture.png)

### 実行時の通信経路

1. ブラウザは`https://todoku-service.com`からAmplify Hosting上のReactアプリを取得します。
2. ReactアプリはECSの公開HTTPSエンドポイントへCookie付きでAPIリクエストを送ります。
3. ECS上のGo APIがセッション、Role、所属、公開範囲を確認します。
4. APIはRDSへSQLを実行し、添付は認可後に非公開S3へ保存・取得します。
5. アプリケーションログとCPUなどのメトリクスはCloudWatchへ送られます。

> 現在、FrontendとAPIは別オリジンです。Safariを含むブラウザのCookie互換性とCSRF耐性を改善するため、将来は`todoku-service.com/api/*`へ統合する方針です。

## デプロイ経路

![Todoku デプロイ経路](deployment-flow.png)

| 対象 | 起点 | ビルド・配布 | 反映方式 |
| --- | --- | --- | --- |
| Frontend | GitHub `main`へのpush | Amplifyが依存関係を取得してVite build | Amplify Hostingへ自動反映 |
| Backend | GitHub `main`のcommit | `linux/amd64`のDocker imageをbuildしてECRへpush | 新しいTask Definition revisionをECSへ反映 |
| 添付基盤 | `deploy/aws/attachment-storage.yml` | AWS CloudFormation | S3とIAM Task Roleを更新 |

BackendはCodePipeline・CodeDeployによるBlue/Green構成ではありません。ECSネイティブのカナリア方式で新revisionの5%を先に稼働させ、3分間のbake後に全体へ昇格します。CloudWatch AlarmまたはDeployment Circuit Breakerが異常を検知した場合は自動ロールバックします。

添付ファイル用S3バケットとECSタスクロールは、[`deploy/aws/attachment-storage.yml`](../../deploy/aws/attachment-storage.yml)でCloudFormation管理しています。ECS、RDS、Amplifyを含む全環境のIaC化とBackendデプロイのCI/CD化は今後の改善項目です。

## セキュリティ上の境界

- APIはセッションCookieで利用者を認証し、Roleと所属情報に基づいて操作・閲覧を制御します。
- RDSはパブリックアクセスを無効化し、ストレージ暗号化と7日間のバックアップを設定しています。
- DB認証情報とセッション秘密情報はSecrets ManagerからECSへ渡します。
- S3はパブリックアクセスを全面的に遮断しています。ブラウザから直接アクセスさせず、APIで認証・閲覧権限を確認してから配信します。
- ECSタスクロールは添付オブジェクトの取得・保存・削除に限定しています。
- 添付のAWS構成図では、ECSのAuto Scaling設定範囲は2〜6タスク、CPU使用率の目標は60%です。図中の稼働1タスクは確認時点の値であり、設定範囲とは区別しています。
