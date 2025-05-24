# Claude Automation System

🤖 **GitHub Issue駆動のClaude自動タスク実行システム**

## 概要

このシステムは、GitHub IssueでClaude AIとやり取りし、自動的にタスクを実行するための分散システムです。1つのIssueにつき1つのDockerコンテナを起動し、完全に分離された環境でタスクを処理します。

## アーキテクチャ

```
GitHub Issue (@claude) → WSL2/Docker Daemon → Container → Task Execution
                                ↓
                         Claude CLI (Host) ← → Agent (Container)
                                ↓
                         GitHub API ← コメント返信・質問
```

## 主要機能

- ✅ **Remote Execution Pattern**: ホストのClaude CLIからコンテナ内実行
- ✅ **1 Issue = 1 Container**: 完全な環境分離
- 🚧 **GitHub Issue監視**: @claudeメンションでの自動起動
- 🚧 **双方向コミュニケーション**: 質問・回答機能
- 🚧 **動的コンテナ選択**: リポジトリに最適なイメージ自動選択

## 現在の状況

### ✅ 完了済み
- Go言語でのPoC実装
- Docker統合とコンテナ管理
- Claude CLI統合
- 基本的なRemote Execution実証

### 🚧 実装中・次のステップ
1. GitHub Issue監視システム
2. Claude CLI権限管理の自動化
3. エラーハンドリング強化
4. LINE連携システム

## クイックスタート

### 前提条件
- Go 1.21+
- Docker
- Claude CLI (認証済み)

### テスト実行
```bash
# 依存関係インストール
go mod tidy

# 基本テスト
./test.sh

# 簡易デモ
go run simple_demo.go
```

## プロジェクト構造

```
├── cmd/
│   ├── orchestrator/    # ホスト側オーケストレーター
│   └── agent/          # コンテナ内エージェント
├── shared/
│   └── workspaces/     # Issue別作業ディレクトリ
├── test.sh            # 統合テストスクリプト
└── simple_demo.go     # 動作確認デモ
```

## 貢献

Issues and Pull Requests are welcome!

## ライセンス

MIT License