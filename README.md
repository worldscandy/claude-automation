# Claude Automation System

GitHub IssueでのClaude自動実行システム - @claudeメンションで自動タスク処理

## 🎯 概要

このシステムは、GitHub Issueで`@claude`とメンションするだけで、自動的にClaude Code（Claude CLI）を起動してタスクを実行し、結果をIssueコメントで返信する自動化システムです。

## ✨ 主要機能

### 🔍 GitHub Issue監視
- **リアルタイム監視**: 30秒間隔でIssue/コメントをチェック
- **スマートなメンション検知**: `@claude`を高精度で検出
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

```
GitHub Issues → Monitor → Orchestrator → Claude CLI → GitHub Comments
     ↓              ↓           ↓             ↓           ↓
  @claude      API Polling   Session    Advanced     Auto Response
  mention      Detection     Management   Features     System
```

## 📁 プロジェクト構造

```
claude-automation/
├── cmd/
│   ├── monitor/      # GitHub Issue監視システム
│   ├── orchestrator/ # Claude CLIタスク実行管理
│   └── agent/        # 将来のコンテナエージェント用
├── workspaces/       # Issue処理用作業領域
├── sessions/         # Claude CLIセッション管理
├── auth/            # 認証ファイル格納
├── setup.sh         # 初期設定スクリプト
├── Dockerfile.claude # Claude実行環境
└── Makefile         # ビルド設定
```

## 🚀 セットアップ

### 1. 前提条件
- **Go 1.21+**: システム要件
- **Claude CLI**: [公式サイト](https://claude.ai/code)からインストール
- **GitHub CLI**: [こちら](https://cli.github.com/)からインストール
- **GitHub Token**: repo権限付きPersonal Access Token

### 2. 初期設定

```bash
# 1. リポジトリクローン
git clone https://github.com/worldscandy/claude-automation.git
cd claude-automation

# 2. 認証設定（自動検出）
./setup.sh

# 3. 環境変数設定
cp .env.example .env
# .envファイルを編集してGITHUB_TOKENを設定

# 4. 依存関係インストール
go mod download

# 5. ビルド
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

### 1. 監視システム起動

```bash
# 開発モード
go run cmd/monitor/main.go

# プロダクションモード
./bin/monitor
```

### 2. GitHub Issueでの使用

任意のIssueまたはコメントで`@claude`とメンションし、実行したいタスクを記述：

```markdown
@claude 以下のタスクをお願いします：
- シンプルなHello Worldプログラムを作成
- テストファイルも含めて実装
- READMEファイルで使用方法を説明
```

### 3. 自動処理フロー

1. **🔍 検知**: 30秒以内にメンションを検出
2. **🚀 開始**: 自動的に処理開始をコメント
3. **⚙️ 実行**: Claude CLIが自律的にタスク処理
4. **✅ 完了**: 結果をIssueにコメントで報告

## 🛠️ 開発・メンテナンス

### ビルドコマンド

```bash
# 全コンポーネントビルド
make build

# 個別ビルド
make monitor      # 監視システム
make orchestrator # タスク実行管理

# クリーンビルド
make clean && make build
```

### テスト実行

```bash
# 基本機能テスト
go test ./...

# 統合テスト
make test

# GitHub API接続テスト
gh auth status
gh repo view worldscandy/claude-automation
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

### プロダクション起動

```bash
# systemdサービス（推奨）
sudo systemctl start claude-automation-monitor

# 直接起動
nohup ./bin/monitor > monitor.log 2>&1 &
```

### 監視・ログ

```bash
# プロセス確認
ps aux | grep monitor

# ログ監視
tail -f monitor.log

# リソース使用量
top -p $(pgrep monitor)
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

### 開発中 🚧
- [ ] **Issue #3**: 動的コンテナ選択
- [ ] **Issue #4**: エラーハンドリング強化
- [ ] **Issue #5**: LINE連携システム

### 将来計画 📅
- [ ] **Webhook対応**: より高速なリアルタイム処理
- [ ] **マルチリポジトリ対応**: 複数リポジトリの一元管理
- [ ] **ダッシュボード**: Web UI for 管理・監視

## 📄 ライセンス

このプロジェクトは [MIT License](LICENSE) の下で公開されています。

## 🙏 謝辞

- **Claude Code**: 強力なAI開発支援
- **GitHub API**: 豊富なプラットフォーム連携
- **Go言語**: 高性能・信頼性の高い実装基盤

---

**🤖 AI-Powered Automation with Claude Code**