# auth-github-pat

nginx の [`auth_request`](https://nginx.org/en/docs/http/ngx_http_auth_request_module.html) ディレクティブ用の認証バックエンドサーバーです。リクエストに含まれる GitHub Personal Access Token (PAT) を検証し、オプションで GitHub Organization / Team への所属を確認します。

## Features

- GitHub PAT の検証 (`GET /user`)
- Organization / Team メンバーシップの確認 (GitHub App 経由)
- インメモリまたは Redis によるキャッシュ
- GHE (GitHub Enterprise) 対応

## Quick Start

```bash
# ビルド
go build -o auth-github-pat .

# PAT 検証のみ
PORT=8080 ./auth-github-pat

# Org/Team チェック付き
PORT=8080 \
  GITHUB_ORG=my-org \
  GITHUB_APP_ID=12345 \
  GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)" \
  GITHUB_APP_INSTALLATION_ID=67890 \
  ./auth-github-pat
```

## Docker

```bash
docker build -t auth-github-pat .
docker run -p 8080:8080 auth-github-pat
```

## Configuration

すべて環境変数で設定します。

| 変数名 | デフォルト | 説明 |
|--------|-----------|------|
| `PORT` | `8080` | サーバーポート |
| `GITHUB_API_URL` | `https://api.github.com` | GitHub API URL (GHE 対応) |
| `GITHUB_ORG` | _(空)_ | 所属チェック対象の Organization |
| `GITHUB_TEAM` | _(空)_ | 所属チェック対象の Team slug (`GITHUB_ORG` が必須) |
| `GITHUB_APP_ID` | _(空)_ | GitHub App ID (`GITHUB_ORG` 設定時に必須) |
| `GITHUB_APP_PRIVATE_KEY` | _(空)_ | GitHub App 秘密鍵 PEM (`GITHUB_ORG` 設定時に必須) |
| `GITHUB_APP_INSTALLATION_ID` | _(空)_ | GitHub App Installation ID (`GITHUB_ORG` 設定時に必須) |
| `CACHE_TYPE` | `memory` | `memory` または `redis` |
| `CACHE_TTL` | `15m` | キャッシュ有効期限 (Go duration 形式: `5m`, `1h` など) |
| `NEGATIVE_CACHE_TTL` | `0` | 認証失敗 (403) 時のキャッシュ有効期限。`0` でネガティブキャッシュ無効 |
| `REDIS_URL` | _(空)_ | Redis 接続 URL (`CACHE_TYPE=redis` 時に必須) |

## Endpoints

### `GET /auth`

nginx `auth_request` のターゲットとなるエンドポイントです。

**リクエスト:**
- `Authorization: Bearer <GitHub PAT>` ヘッダが必要

**レスポンス:**
| Status | 意味 |
|--------|------|
| `200` | 認証成功。`X-Auth-User` ヘッダに GitHub ユーザー名を設定 |
| `401` | トークンが無効、または `Authorization` ヘッダが欠損 |
| `403` | トークンは有効だが、指定された Org/Team に所属していない |

### `GET /healthz`

ヘルスチェック用。常に `200` を返します。

## Auth Flow

1. `Authorization: Bearer <token>` ヘッダから PAT を抽出
2. PAT の SHA-256 ハッシュでキャッシュを検索
   - ヒット → キャッシュされた結果に基づき `200` / `403` を返す
3. キャッシュミス時、PAT を使って GitHub API `GET /user` でユーザーを検証
   - 無効なトークン → `401` を返す（キャッシュしない）
4. `GITHUB_ORG` 設定時、GitHub App Installation Token で `GET /orgs/{org}/members/{username}` を呼び出し Org 所属を確認
   - 非所属 → `NEGATIVE_CACHE_TTL` が設定されていればキャッシュに保存し、`403` を返す
5. `GITHUB_TEAM` 設定時、GitHub App Installation Token で `GET /orgs/{org}/teams/{team}/members/{username}` を呼び出し Team 所属を確認
   - 非所属 → `NEGATIVE_CACHE_TTL` が設定されていればキャッシュに保存し、`403` を返す
6. 検証結果をキャッシュに保存し、`200` を返す

## nginx Configuration Example

```nginx
server {
    listen 80;

    location /private/ {
        auth_request /auth;
        auth_request_set $auth_user $upstream_http_x_auth_user;
        proxy_set_header X-Auth-User $auth_user;
        proxy_pass http://backend;
    }

    location = /auth {
        internal;
        proxy_pass http://auth-github-pat:8080/auth;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
        proxy_set_header Authorization $http_authorization;
    }
}
```

## GitHub App Setup

Org/Team メンバーシップチェックを使用する場合、GitHub App の作成とインストールが必要です。

1. GitHub App を作成
   - **Organization members**: Read-only 権限を付与
2. 秘密鍵を生成しダウンロード
3. 対象の Organization にアプリをインストール
4. App ID、Installation ID、秘密鍵を環境変数に設定

## License

MIT
