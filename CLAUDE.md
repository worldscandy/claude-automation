# Claude Automation System - 現在の状況と引き継ぎ情報

## 📋 プロジェクト概要
GitHub Issueで@claudeメンションを検知し、自動的にDockerコンテナ内でタスクを実行するシステムの構築

### リポジトリ
- URL: https://github.com/worldscandy/claude-automation
- 初期実装完了済み

## ✅ 完了したタスク

### 1. **基本アーキテクチャの実装**
- Go言語でのPoC実装完了
- 1 Issue = 1 Container の設計実装
- Remote Executionパターンの動作確認

### 2. **認証管理システム**
- ホストのClaude認証情報を安全に管理する仕組み構築
- `setup.sh` - 自動検出/手動設定の2モード対応
- 認証ファイルテンプレート作成済み
  - `.claude.json.template`
  - `.claude.credentials.json.template`
  - `.env.example`

### 3. **コンテナ内でのClaude CLI動作確認**
- 認証情報のマウントで動作確認
- 計算処理（579 = 123 + 456）成功
- 必要なファイル構造:
  - `auth/.claude.json` - Claude設定
  - `auth/.credentials.json` - OAuthトークン

## 🚧 現在の課題

### 1. **Claude CLIの権限管理**
- ファイル作成時に手動権限付与が必要
- `--yes`オプションが存在しない
- `--dangerously-skip-permissions`オプションの検証が必要

### 2. **GitHub Issue監視システム（未完成）**
- `cmd/monitor/main.go` - 基本実装済み
- 実際のコンテナ起動との統合が未実装

## 📁 プロジェクト構造
```
claude-automation/
├── cmd/
│   ├── orchestrator/    # ホスト側制御
│   ├── agent/          # コンテナ内エージェント
│   └── monitor/        # GitHub Issue監視（作成中）
├── auth/               # 認証ファイル（.gitignore）
├── setup.sh           # 初期設定スクリプト
├── test-*.sh          # 各種テストスクリプト
└── go.mod             # Go依存関係
```

## 🔜 次のステップ

### 優先度高
1. **Claude CLI権限自動化の解決**
   - Dockerコンテナ内での`--dangerously-skip-permissions`検証
   - または対話的承認の自動化方法検討

2. **GitHub Issue監視とコンテナ起動の統合**
   - `cmd/monitor/main.go`とorchestrator連携
   - Issue IDとコンテナのマッピング実装

3. **エンドツーエンドテスト**
   - GitHub Issueで@claudeメンション
   - コンテナ起動
   - タスク実行
   - 結果をIssueにコメント

### 優先度中
4. **動的コンテナ選択（Issue #3）**
   - 言語検出によるイメージ自動選択

5. **エラーハンドリング強化（Issue #4）**
   - ログシステム
   - エラー時のIssue通知

### 優先度低
6. **LINE連携（Issue #5）**
   - モバイルからのIssue作成

## 🔑 重要な認証情報
- `.env`ファイルにGitHubトークンが必要
- Claude認証は`setup.sh`で自動抽出可能
- トークン有効期限: 2025年5月24日

## 💡 引き継ぎポイント
1. Claude CLIの権限問題が最優先課題
2. 認証システムは完成しているので、そのまま使用可能
3. GitHub Issue監視の基本実装は完了、統合作業が必要

## 🐛 既知の問題と解決策

### Claude CLIコンテナ実行時の注意点
1. **認証ファイルのマウント方法**
   ```bash
   -v $(pwd)/auth/.claude.json:/root/.claude.json
   -v $(pwd)/auth:/root/.claude
   ```

2. **必要な環境変数**
   ```bash
   -e SHELL=/bin/bash
   ```

3. **Alpine Linuxでの問題**
   - `env -S`オプションがBusyBoxで非対応
   - 解決策: `node:20`（Debian系）を使用、またはcoreutilsインストール

### テスト済みの動作確認コマンド
```bash
# 計算テスト（動作確認済み）
echo "What is 123 + 456? Reply with just the number." | claude --print

# ファイル作成（権限問題あり）
echo "Create a file..." | claude --print --dangerously-skip-permissions
```

## 📝 開発メモ

### GitHub Issues
- #1: GitHub Issue監視システム
- #2: Claude CLI権限管理 
- #3: 動的コンテナ選択
- #4: エラーハンドリング強化
- #5: LINE連携システム
- #6: Unix Socket通信修正（解決済み）

### セットアップ手順
1. `./setup.sh`を実行（オプション1で自動検出）
2. GitHubトークンを`.env`に追加
3. `make build`でビルド
4. 各種テストスクリプトで動作確認