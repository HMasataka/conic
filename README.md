# Conic

WebSocket接続を使用したリアルタイムピアツーピア通信を促進するGo言語ベースのWebRTCシグナリングサーバーです。

## 機能

- WebRTC接続用のWebSocketベースシグナリングサーバー
- クライアント登録とメッセージルーティング
- SDP（Session Description Protocol）交換のサポート
- NAT越えのためのICE候補交換
- Goチャンネルを使用した並行メッセージ処理
- クリーンなインターフェースベースのアーキテクチャ

## 必要要件

- Go 1.24.5 以降
- 依存関係ダウンロード用のインターネット接続

## インストール

```bash
# リポジトリをクローン
git clone https://github.com/HMasataka/conic.git
cd conic

# 依存関係をインストール
go mod tidy
```

## 使用方法

### サーバーの起動

WebRTCシグナリングサーバーを開始します：

```bash
# Taskを使用
task server

# またはGoで直接実行
go run cmd/server/main.go
```

サーバーは`localhost:3000`で起動し、`/ws`でWebSocket接続を受け付けます。

### クライアントの起動

シグナリングサーバーにクライアントを接続します：

```bash
# Taskを使用
task client

# またはGoで直接実行
go run cmd/client/main.go
```

異なるサーバーアドレスを指定できます：

```bash
go run cmd/client/main.go -addr "localhost:8080"
```

## API

### WebSocketエンドポイント

- **URL**: `ws://localhost:3000/ws`
- **プロトコル**: JSONメッセージを使用するWebSocket

### メッセージタイプ

#### クライアント登録

```json
{
  "Type": "register"
}
```

#### クライアント登録解除

```json
{
  "Type": "unregister",
  "Raw": "{\"ID\": \"client-id\"}"
}
```

#### SDPオファー/アンサー送信

```json
{
  "Type": "sdp",
  "Raw": "{\"ID\": \"sender-id\", \"TargetID\": \"receiver-id\", \"SessionDescription\": {...}}"
}
```

#### ICE候補送信

```json
{
  "Type": "candidate",
  "Raw": "{\"ID\": \"sender-id\", \"TargetID\": \"receiver-id\", \"Candidate\": \"candidate-string\"}"
}
```

## アーキテクチャ

### コアコンポーネント

- **Hub**: クライアント接続を管理し、メッセージをルーティングする中央メッセージルーティングシステム
- **WebSocket Server**: WebSocket接続とプロトコルアップグレードを処理
- **Client**: シグナリングサーバーへの接続用WebSocketクライアント実装
- **Handshake**: ICE候補処理を含むWebRTCピア接続管理

### 依存関係

- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket実装
- [Pion WebRTC](https://github.com/pion/webrtc) - Pure Go WebRTC実装
- [Chi Router](https://github.com/go-chi/chi) - 軽量HTTPルーター
- [XID](https://github.com/rs/xid) - グローバル一意ID生成器

## 開発

### 利用可能なコマンド

```bash
# サーバーを起動
task server

# クライアントを起動
task client

# シグナルアプリを起動
task signal

# 利用可能なすべてのタスクを表示
task --list

# プロジェクトをビルド
task build

# テストを実行（利用可能な場合）
task test

# コードをフォーマット
task fmt

# コードの問題をチェック
task vet

# 依存関係を整理
task tidy

# ビルド成果物をクリーンアップ
task clean

# 開発ツールをインストール
task install-tools

# ホットリロード付きでサーバーを起動（airが必要）
task dev-server
```

### プロジェクト構造

```bash
/
├── cmd/
│   ├── client/     # クライアントアプリケーション
│   ├── server/     # サーバーアプリケーション
│   └── signal/     # シグナルアプリケーション
├── client.go       # クライアント実装
├── hub.go          # メッセージハブとルーティング
├── websocket.go    # WebSocketサーバー処理
├── handshake.go    # WebRTCハンドシェイク管理
├── go.mod          # Goモジュール定義
└── Taskfile.yml    # タスクランナー設定
```

## ライセンス

このプロジェクトは、リポジトリで指定された条件の下で利用できます。

## 貢献

1. リポジトリをフォーク
2. フィーチャーブランチを作成（`git checkout -b feature/amazing-feature`）
3. 変更をコミット（`git commit -m 'Add some amazing feature'`）
4. ブランチにプッシュ（`git push origin feature/amazing-feature`）
5. プルリクエストを開く
