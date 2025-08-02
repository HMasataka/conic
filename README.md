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
  "type": "register"
}
```

#### クライアント登録解除

```json
{
  "type": "unregister",
  "raw": "{\"ID\": \"client-id\"}"
}
```

#### SDPオファー/アンサー送信

```json
{
  "type": "sdp",
  "raw": "{\"ID\": \"sender-id\", \"TargetID\": \"receiver-id\", \"SessionDescription\": {...}}"
}
```

#### ICE候補送信

```json
{
  "type": "candidate",
  "raw": "{\"ID\": \"sender-id\", \"TargetID\": \"receiver-id\", \"Candidate\": \"candidate-string\"}"
}
```

## アーキテクチャ

### コアコンポーネント

- **Hub**: クライアント接続を管理し、メッセージをルーティングする中央メッセージルーティングシステム
- **WebSocket Server**: WebSocket接続とプロトコルアップグレードを処理
- **Client**: シグナリングサーバーへの接続用WebSocketクライアント実装
- **Handshake**: ICE候補処理を含むWebRTCピア接続管理

### WebRTCシグナリングプロセス

```mermaid
sequenceDiagram
    participant C1 as Client 1
    participant S as Signaling Server
    participant H as Hub
    participant C2 as Client 2

    Note over C1, C2: 1. 接続とクライアント登録

    C1->>+S: WebSocket接続 (ws://localhost:3000/ws)
    S->>S: WebSocket升級
    S->>+H: Socket作成とサービス開始

    C1->>S: {"type": "register"}
    S->>S: RegisterHandler処理
    S->>H: RegisterRequest {ID, Client}
    H->>H: clients[id] = client
    S-->>C1: RegisterResponse {ID}
    Note over C1: クライアントIDを受信・保存

    C2->>+S: WebSocket接続
    C2->>S: {"type": "register"}
    S->>H: RegisterRequest {ID, Client}
    H->>H: clients[id] = client
    S-->>C2: RegisterResponse {ID}

    Note over C1, C2: 2. WebRTCハンドシェイク開始

    C1->>C1: InitHandshake(config)
    C1->>C1: CreateDataChannel("test")
    C1->>C1: peerConnection.CreateOffer()

    C1->>S: {"type": "sdp", "raw": SDPRequest}
    Note right of S: SDPRequest: {ID, TargetID, SessionDescription}
    S->>S: SDPHandler処理
    S->>H: MessageRequest {ID, TargetID, Message}
    H->>C2: SDP Offerメッセージ転送

    Note over C2: 3. Answer作成と応答

    C2->>C2: handleSDP() - Offer受信
    C2->>C2: handshake.SetRemoteDescription(offer)
    C2->>C2: peerConnection.CreateAnswer()
    C2->>C2: handshake.SetLocalDescription(answer)

    C2->>S: {"type": "sdp", "raw": SDPRequest}
    Note right of S: SDPRequest: {ID, TargetID, SessionDescription}
    S->>H: MessageRequest {ID, TargetID, Message}
    H->>C1: SDP Answerメッセージ転送

    Note over C1, C2: 4. ICE候補交換

    C1->>C1: OnICECandidate イベント
    C1->>S: {"type": "candidate", "raw": CandidateRequest}
    S->>S: CandidateHandler処理
    S->>H: MessageRequest {ID, TargetID, Message}
    H->>C2: ICE候補メッセージ転送
    C2->>C2: handshake.AddIceCandidate()

    C2->>C2: OnICECandidate イベント
    C2->>S: {"type": "candidate", "raw": CandidateRequest}
    S->>H: MessageRequest {ID, TargetID, Message}
    H->>C1: ICE候補メッセージ転送
    C1->>C1: handshake.AddIceCandidate()

    Note over C1, C2: 5. データチャネル通信

    Note over C1: データチャネル確立後
    C1->>C1: dataChannel.OnOpen()
    C2->>C2: OnDataChannel() イベント

    C1->>C1: dataChannel.Send(data)
    Note right of C1: P2P直接通信
    C1-->>C2: データ送信 (P2P)

    Note over C1, C2: または、シグナリング経由でのデータ送信
    C1->>S: {"type": "data_channel", "raw": DataChannelRequest}
    S->>H: MessageRequest
    H->>C2: データチャネルメッセージ転送
```

### アーキテクチャ構成図

```mermaid
graph TB
    subgraph "Client Side"
        C1[Client 1]
        C2[Client 2]

        subgraph "Client Components"
            WS[WebSocket Connection]
            HS[Handshake Manager]
            DC[DataChannel Manager]
        end
    end

    subgraph "Server Side"
        subgraph "Signal Package"
            WSS[WebSocket Server]
            SOC[Socket Handler]

            subgraph "Message Handlers"
                RH[RegisterHandler]
                UH[UnregisterHandler]
                SH[SDPHandler]
                CH[CandidateHandler]
                DH[DataChannelHandler]
            end
        end

        subgraph "Hub Package"
            HUB[Hub]
            REG[Register Channel]
            UNREG[Unregister Channel]
            MSG[Message Channel]
            CLIENTS[Client Registry]
        end
    end

    subgraph "WebRTC Layer"
        PC1[PeerConnection 1]
        PC2[PeerConnection 2]
        ICE[ICE Candidates]
        SDP[SDP Exchange]
    end

    C1 -.->|WebSocket| WSS
    C2 -.->|WebSocket| WSS

    WSS --> SOC
    SOC --> RH
    SOC --> UH
    SOC --> SH
    SOC --> CH
    SOC --> DH

    RH --> REG
    UH --> UNREG
    SH --> MSG
    CH --> MSG
    DH --> MSG

    REG --> HUB
    UNREG --> HUB
    MSG --> HUB

    HUB --> CLIENTS

    C1 --> PC1
    C2 --> PC2
    PC1 -.->|P2P Data| PC2

    PC1 --> ICE
    PC2 --> ICE
    PC1 --> SDP
    PC2 --> SDP
```

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
