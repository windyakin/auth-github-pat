# CLAUDE.md

## Project Overview

nginx の `auth_request` ディレクティブ用の認証バックエンドサーバー。GitHub Personal Access Token (PAT) を検証し、オプションで GitHub Org/Team の所属を確認する。

## Tech Stack

- Go (標準ライブラリの `net/http` ベース、Web フレームワーク不使用)
- `github.com/golang-jwt/jwt/v5` — GitHub App JWT 生成
- `github.com/redis/go-redis/v9` — Redis キャッシュ
- `github.com/alicebob/miniredis/v2` — Redis テスト用

## Architecture

```
config/   — 環境変数からの設定読み込み
cache/    — Cache インターフェースとインメモリ/Redis 実装
github/   — GitHub API クライアント (PAT検証) と GitHub App クライアント (Org/Team確認)
handler/  — HTTP ハンドラ (/auth エンドポイント)
main.go   — エントリポイント
```

### Auth Flow

1. `Authorization: Bearer <token>` から PAT を抽出
2. SHA-256 ハッシュでキャッシュ検索
3. キャッシュミス時: PAT で `GET /user` → Org/Team チェックは GitHub App Installation Token で実行
4. 無効トークンはキャッシュしない（キャッシュ汚染防止）

## Commands

```bash
# テスト
go test ./...

# ビルド
go build -o auth-github-pat .

# 実行
PORT=8080 go run .
```

## Testing

- GitHub API テスト: `httptest.NewServer` によるモック
- Redis テスト: `miniredis` によるインプロセスモック
- ハンドラテスト: インターフェースのモック実装を注入
