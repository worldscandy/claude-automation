# Claude Automation System

GitHub IssueでのClaude自動実行システム - @claude-codeメンションで自動タスク処理

## 🎯 概要

このシステムは、GitHub Issueで`@claude-code`とメンションするだけで、自動的にClaude Code（Claude CLI）を起動してタスクを実行し、結果をIssueコメントで返信する自動化システムです。

## ✨ 主要機能

### 🔍 GitHub Issue監視
- **リアルタイム監視**: 30秒間隔でIssue/コメントをチェック
- **スマートなメンション検知**: `@claude-code`を高精度で検出
- **自動タスク抽出**: Issue本文・コメントからタスク内容を解析

### 🤖 Claude CLI統合
- **自律実行**: `--max-turns`による段階的タスク処理
- **セッション管理**: `--continue`で複数ターン対応
- **詳細ログ**: `--verbose`で完全な実行履歴
- **構造化出力**: `--output-format json`でデータ処理

### 🔄 自動応答システム
- **進捗報告**: 処理開始・進行状況を自動コメント
- **結果通知**: 完了時に結果をIssue返信
- **エラーハンドリング**: 失敗時の詳細エラー報告

## 🏗️ アーキテクチャ

### Container Orchestration System (Kubernetes Native)

```
GitHub Issues → Monitor Pod → Worker Pod (Kubernetes) → Claude CLI → GitHub Comments
     ↓              ↓              ↓                    ↓           ↓
  @claude-code      API Polling    Dynamic Pod          Real Claude    Auto Response
  mention      Detection      Creation             CLI Execution   System
```

### システム特徴
- **🐳 Kubernetes Native**: Docker-in-DockerからKubernetes Podへ完全移行
- **⚡ Dynamic Scaling**: Issue毎の独立Worker Pod自動作成
- **🔒 Security**: Pod-level分離・RBAC権限管理
- **🔄 Auto Cleanup**: タスク完了時のPod自動削除

## 📁 プロジェクト構造

```
claude-automation/
├── cmd/
│   ├── monitor/      # GitHub Issue監視システム (Kubernetes Pod)
│   ├── orchestrator/ # Claude CLIタスク実行管理 (Worker Pod管理)
│   ├── agent/        # 将来のコンテナエージェント用
│   └── token-renewal/ # OAuth Token自動更新システム
├── pkg/
│   ├── container/    # Container Manager (Pod動的作成・管理)
│   ├── kubernetes/   # Kubernetes Client (SPDY Executor・API統合)
│   └── auth/         # 認証システム (Token管理・永続化)
├── docker/           # Container Images
│   ├── Dockerfile    # Claude CLI実行環境 (Alpine Linux)
│   └── .dockerignore # Build最適化設定
├── deployments/      # Kubernetes Manifests
│   └── monitor-deployment.yaml # Monitor Pod配置設定
├── test/integration/ # End-to-End統合テスト
│   ├── orchestrator/ # Container Orchestration動作確認
│   ├── auth/         # 認証システムテスト
│   └── auth-k8s/     # Kubernetes認証統合テスト
├── docs/             # 技術文書
│   └── TOKEN-RENEWAL.md # Token更新システム仕様
├── workspaces/       # Issue処理用作業領域
├── sessions/         # Claude CLIセッション管理
├── config/           # 設定ファイル
│   └── repo-mapping.yaml # リポジトリマッピング設定
├── scripts/          # 運用スクリプト
│   └── token-renewal.sh # Token更新自動化
├── docker-compose.yml # 開発環境構築
├── entrypoint.sh     # Container起動スクリプト
└── Makefile         # ビルド・デプロイ設定
```

## 🚀 セットアップ

### 1. 前提条件
- **Go 1.21+**: システム要件
- **minikube**: 開発環境Kubernetes（推奨）
- **Docker**: Container Image build用
- **Claude CLI**: [公式サイト](https://claude.ai/code)からインストール
- **GitHub CLI**: [こちら](https://cli.github.com/)からインストール
- **GitHub Token**: repo権限付きPersonal Access Token

### 2. minikube環境構築

```bash
# 1. リポジトリクローン
git clone https://github.com/worldscandy/claude-automation.git
cd claude-automation

# 2. minikube起動・Docker環境設定
minikube start
eval $(minikube docker-env)  # minikubeのDockerデーモン使用
minikube dashboard  # Web UI（オプション）

# 3. Claude CLI実行環境Docker Image構築
docker build -f docker/Dockerfile -t claude-automation-claude .
minikube image load claude-automation-claude

# 4. 認証設定（自動検出）
./setup.sh

# 5. 環境変数設定
cp .env.example .env
# .envファイルを編集してGITHUB_TOKENを設定

# 6. 依存関係インストール
go mod download

# 7. ビルド
make build
```

### 3. 認証ファイル確認

正常にセットアップされていれば、以下のファイルが作成されます：

```
auth/
├── .claude.json      # Claude設定
└── .credentials.json # OAuthトークン
```

## 📖 使用方法

### 1. minikube監視システム起動

```bash
# 開発モード（ローカル実行）
go run cmd/monitor/main.go

# minikubeデプロイ
minikube kubectl -- apply -f deployments/monitor-deployment.yaml

# Pod状況確認
minikube kubectl -- get pods -l app=claude-automation-monitor
minikube kubectl -- logs -f deployment/claude-automation-monitor
```

### 2. GitHub Issueでの使用

任意のIssueまたはコメントで`@claude-code`とメンションし、実行したいタスクを記述：

```markdown
@claude-code 以下のタスクをお願いします：
- シンプルなHello Worldプログラムを作成
- テストファイルも含めて実装
- READMEファイルで使用方法を説明
```

### 3. Container Orchestration自動処理フロー

1. **🔍 検知**: Monitor Podが30秒以内にメンション検出
2. **🐳 Pod作成**: Issue専用Worker Pod動的作成
3. **🚀 開始**: 自動的に処理開始をコメント
4. **⚙️ 実行**: Pod内Claude CLIが自律的にタスク処理
5. **✅ 完了**: 結果をIssueにコメント・Pod自動削除

## 🛠️ 開発・メンテナンス

### ビルド・デプロイコマンド

```bash
# 全コンポーネントビルド
make build

# 個別ビルド
make monitor      # 監視システム
make orchestrator # タスク実行管理
make token-renewal # Token更新システム

# Container Image構築
make docker-build
minikube image load claude-automation-claude

# minikubeデプロイ
minikube kubectl -- apply -f deployments/monitor-deployment.yaml

# クリーンビルド
make clean && make build
```

### テスト実行

```bash
# 基本機能テスト
go test ./...

# Container Orchestration統合テスト
go run test/integration/orchestrator/main.go

# 認証システムテスト
go run test/integration/auth/main.go
go run test/integration/auth-k8s/main.go

# GitHub API接続テスト
gh auth status
gh repo view worldscandy/claude-automation

# minikube動作確認
minikube kubectl -- get pods
minikube kubectl -- logs -f deployment/claude-automation-monitor
```

### トラブルシューティング

#### Claude CLI認証エラー
```bash
# 認証状態確認
claude --version

# 再設定
./setup.sh
```

#### GitHub API エラー
```bash
# トークン確認
gh auth status

# トークン再設定
gh auth login
```

#### 監視システムが反応しない
```bash
# ログ確認
go run cmd/monitor/main.go

# minikube Pod確認
minikube kubectl -- get pods
minikube kubectl -- describe pod <pod-name>

# 環境変数確認
echo $GITHUB_TOKEN
```

## 🔧 設定

### 環境変数

| 変数名 | 説明 | デフォルト |
|--------|------|------------|
| `GITHUB_TOKEN` | GitHub Personal Access Token | 必須 |
| `GITHUB_OWNER` | リポジトリオーナー | `worldscandy` |
| `GITHUB_REPO` | リポジトリ名 | `claude-automation` |

### 設定ファイル

- **`.env`**: 環境変数設定
- **`auth/.claude.json`**: Claude CLI設定
- **`auth/.credentials.json`**: OAuth認証情報

## 🚦 運用

### minikubeプロダクション運用

```bash
# minikubeデプロイ
minikube kubectl -- apply -f deployments/monitor-deployment.yaml

# スケーリング
minikube kubectl -- scale deployment claude-automation-monitor --replicas=3

# ローリングアップデート
minikube kubectl -- rollout restart deployment/claude-automation-monitor
minikube kubectl -- rollout status deployment/claude-automation-monitor

# サービス公開（オプション）
minikube kubectl -- expose deployment claude-automation-monitor --type=LoadBalancer --port=8080
minikube service claude-automation-monitor
```

### 監視・ログ

```bash
# Pod状況確認
minikube kubectl -- get pods -l app=claude-automation-monitor
minikube kubectl -- describe pod <pod-name>

# ログ監視
minikube kubectl -- logs -f deployment/claude-automation-monitor
minikube kubectl -- logs -f <worker-pod-name>

# リソース使用量
minikube kubectl -- top pods
minikube kubectl -- top nodes

# 動的Worker Pod監視
watch minikube kubectl -- get pods -l type=worker
```

## 🤝 コントリビューション

1. **Fork** このリポジトリ
2. **Feature branch** 作成: `git checkout -b feature/amazing-feature`
3. **Commit** 変更: `git commit -m 'Add amazing feature'`
4. **Push** ブランチ: `git push origin feature/amazing-feature`
5. **Pull Request** 作成

## 📋 ロードマップ

### 完了済み ✅
- [x] **Issue #1**: GitHub Issue監視システム
- [x] **Issue #2**: Claude CLI権限管理
- [x] **Issue #6**: Unix Socket通信（不要判定）
- [x] **Issue #9**: Container Orchestration System
  - [x] **Issue #11**: Kubernetes Native移行・Docker-in-Docker権限問題解決
  - [x] **Issue #12**: Dockerfile.claude base Worker Container統合
  - [x] **Issue #13**: 実際のClaude CLI統合とContainer内実行
  - [x] **Issue #14**: End-to-End Container Orchestration動作確認
  - [x] **Issue #16**: Claude CLI OAuth Token自動更新・認証永続化システム
  - [x] **Issue #17**: Claude CLI Alpine Linux互換性問題・実行環境修正

### 開発中 🚧
- [ ] **Issue #3**: 動的コンテナ選択（Kubernetes Native実装済み）
- [ ] **Issue #4**: エラーハンドリング強化
- [ ] **Issue #5**: LINE連携システム

### 将来計画 📅
- [ ] **Webhook対応**: より高速なリアルタイム処理
- [ ] **マルチリポジトリ対応**: 複数リポジトリの一元管理
- [ ] **ダッシュボード**: Web UI for 管理・監視
- [ ] **Auto Scaling**: Horizontal Pod Autoscaler対応
- [ ] **Multi-Cluster**: 複数Kubernetesクラスター対応

## 📄 ライセンス

このプロジェクトは [MIT License](LICENSE) の下で公開されています。

## 🙏 謝辞

- **Claude Code**: 強力なAI開発支援
- **GitHub API**: 豊富なプラットフォーム連携
- **Go言語**: 高性能・信頼性の高い実装基盤

---

**🤖 AI-Powered Automation with Claude Code**