# 🔐 Claude CLI Token Renewal Guide

## 概要

Claude Automation SystemでClaude CLIのOAuth tokenが期限切れになった場合の更新手順です。

## 🚨 Token期限切れの検出

システムが以下のアラートを表示した場合、token更新が必要です：

```
🚨 **URGENT: Claude CLI Token EXPIRED!**

The Claude CLI authentication token has expired. Container orchestration will fail until renewed.

**Required Action:**
1. Run `claude login` on the host system
2. Follow the browser authentication flow
3. Update `.env-secret` with new token values
4. Restart container orchestration system
```

## 🛠️ Token更新方法

### 方法1: 専用コンテナを使用（推奨）

1. **Token再取得コンテナを起動**:
   ```bash
   make token-renewal
   ```

2. **コンテナ内でClaude CLI認証**:
   ```bash
   claude login
   ```

3. **ブラウザ認証フローを実行**:
   - 表示されたURLをブラウザで開く
   - Claude Pro Max accountでログイン
   - 認証コードをコピー
   - ターミナルに認証コードを貼り付け

4. **Token情報を抽出**:
   ```bash
   /app/scripts/token-renewal.sh
   ```

5. **出力された設定をホストの`.env-secret`にコピー**:
   ```bash
   # Claude Authentication Configuration
   CLAUDE_ACCESS_TOKEN=sk-ant-oat01-...
   CLAUDE_TOKEN_EXPIRES_AT=1748089875160
   CLAUDE_USER_ID=15a757bc...
   # ... (その他の設定)
   ```

6. **Container Orchestrationシステムを再起動**

### 方法2: ホストシステムで直接実行

1. **Claude CLIでログイン**:
   ```bash
   claude login
   ```

2. **認証ファイルから値を手動抽出**:
   - `~/.claude.json` からユーザー情報
   - `~/.claude/.credentials.json` からtoken情報

3. **`.env-secret`ファイルを更新**

## 📁 関連ファイル

### コンテナファイル
- `Dockerfile.token-renewal` - Token更新用コンテナ定義
- `scripts/token-renewal.sh` - Token抽出ヘルパースクリプト
- `cmd/token-renewal/main.go` - Token更新管理コマンド

### 設定ファイル
- `.env-secret` - 認証情報格納ファイル（ホスト）
- `~/.claude.json` - Claude CLIユーザー設定
- `~/.claude/.credentials.json` - OAuth認証情報

## 🔄 自動化フロー

```
Token期限切れ検出 → アラート生成 → Token更新コンテナ起動
                                         ↓
Container再起動 ← .env-secret更新 ← 新Token抽出
```

## 🛡️ セキュリティ考慮事項

1. **Token機密性**: `.env-secret`ファイルは`.gitignore`で除外済み
2. **権限管理**: 認証ファイルは600パーミッションで保護
3. **コンテナ分離**: Token更新は専用コンテナで実行
4. **有効期限管理**: 自動期限監視・事前アラート機能

## 🚀 次回の期限切れ予防

- **24時間前アラート**: 自動GitHub Issue作成
- **4時間前アラート**: 最終警告
- **1時間前アラート**: 緊急対応要求

定期的なtoken更新により、システムの無停止運用が可能です。

## ❓ トラブルシューティング

### Token更新に失敗する場合

1. **Claude Pro Maxアカウント確認**: 有効なsubscriptionが必要
2. **ネットワーク接続確認**: ブラウザ認証にインターネット接続が必要
3. **Docker環境確認**: コンテナ実行にDocker Desktopが必要

### システムが認証を認識しない場合

1. **`.env-secret`ファイル形式確認**: 正しいkey=value形式
2. **Container再起動**: 新しい認証情報の読み込み確認
3. **認証テスト実行**: `make auth-test`で動作確認