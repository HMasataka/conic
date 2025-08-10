# Conic

WebSocket接続を使用したリアルタイムピアツーピア通信を促進するGo言語ベースのWebRTCシグナリングサーバーです。

## 機能

- WebRTC接続用のWebSocketベースシグナリングサーバー
- クライアント登録とメッセージルーティング
- SDP（Session Description Protocol）交換のサポート
- NAT越えのためのICE候補交換
- **データチャネルによるP2P通信サポート**
- **自動WebRTCハンドシェイク処理（Offer/Answer）**
- **リアルタイムチャット機能**
- Goチャンネルを使用した並行メッセージ処理
- クリーンなインターフェースベースのアーキテクチャ

## 必要要件

- Go 1.24.5 以降
- 依存関係ダウンロード用のインターネット接続

## 使用方法

### サンプル作成

```bash
task generate-audio
```

### P2P通信デモ

完全なWebRTCピアツーピア通信を体験できます：

```bash
# インタラクティブP2Pクライアントを起動
task datachannel

# オファー側として起動
task datachannel-offer
# または
task datachannel -- -role=offer

# アンサー側として起動
task datachannel-answer
# または
task datachannel -- -role=answer
```

P2P通信デモの使い方：

1. **サーバー起動**: まずシグナリングサーバーを起動

```bash
task signal
```

1. **2つのクライアントを起動**: 別々のターミナルで

```bash
# ターミナル1: オファー側
go run cmd/datachannel/main.go

# ターミナル2: アンサー側
go run cmd/datachannel/main.go -role=answer
```

1. **P2P接続の確立**: オファー側でターゲットのピアIDを入力すると自動的にWebRTCハンドシェイクが開始されます

1. **リアルタイム通信**: 接続が確立されるとデータチャネル経由でのリアルタイム通信が可能になります

#### P2Pデモの特徴

```bash
task datachannel
```

利用可能なコマンド：

- `offer <peer_id>` - 指定したピアにWebRTCオファーを作成・送信
- `channel <label>` - 新しいデータチャネルを作成
- `send <label> <message>` - データチャネル経由でメッセージ送信
- `list` - アクティブなデータチャネルを一覧表示
- `quit` - 終了

## クイックスタート

P2P通信を素早く体験するには：

```bash
# ターミナル1: サーバー起動
task signal

# ターミナル2: オファー側クライアント起動
task datachannel-offer

# ターミナル3: アンサー側クライアント起動
task datachannel-answer
```

1. オファー側でアンサー側のピアIDを入力
2. WebRTC接続が自動確立
3. データチャネル経由でリアルタイム通信開始！

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

#### データチャネルメッセージ送信

```json
{
  "type": "data_channel",
  "raw": "{\"ID\": \"sender-id\", \"TargetID\": \"receiver-id\", \"Label\": \"channel-name\", \"Data\": \"base64-encoded-data\"}"
}
```

## アーキテクチャ

### コアコンポーネント

#### Domain Layer (`domain/`)

- **Hub Interface**: メッセージルーティングの契約定義
- **Client Interface**: クライアント接続の契約定義
- **Message Types**: JSONマーシャリング対応のメッセージ型定義
- **統計情報**: Hub統計追跡機能

#### Internal Packages (`internal/`)

- **WebRTC (`internal/webrtc/`)**
  - `PeerConnection`: WebRTCピア接続管理（統計、ICE候補キューイング、エラー処理）
  - `DataChannel`: データチャネル管理（統計、イベントハンドラー、スレッドセーフ操作）
- **Transport (`internal/transport/`)**
  - `Client`: サーバーサイドクライアント表現
  - `WebSocket Connection`: WebSocket接続管理・アップグレード処理
- **Protocol (`internal/protocol/`)**
  - `Message Handlers`: メッセージタイプ別ハンドラー（register, SDP, candidate, data_channel）
  - `Router`: メッセージルーティング設定

#### Hub Implementation (`hub/`)

- **Central Routing**: クライアント登録・登録解除管理
- **Message Broadcasting**: 接続クライアント間のメッセージルーティング
- **Connection Tracking**: 接続統計と管理
- **Concurrent Processing**: Goチャンネルを使用した並行処理

#### Signal Package (`signal/`)

- **WebSocket Server**: WebSocket接続とアップグレード処理
- **P2P管理**: 完全なWebRTCハンドシェイクワークフロー
- **Data Channel Management**: データチャネルとピア接続管理

#### Logging (`logging/`)

- **構造化ログ**: Go `slog`パッケージ使用
- **コンテキスト対応**: コンテキスト認識ログ
- **設定可能**: レベルとフォーマット（JSON/テキスト）設定可能

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
        end

        subgraph "Internal Packages"
            subgraph "Transport Layer"
                WS[WebSocket Connection]
                CLI[Client Manager]
            end

            subgraph "Protocol Layer"
                RTR[Message Router]
                RH[RegisterHandler]
                UH[UnregisterHandler]
                SH[SDPHandler]
                CH[CandidateHandler]
                DH[DataChannelHandler]
            end

            subgraph "WebRTC Layer"
                PC[PeerConnection]
                DC[DataChannel]
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
    SOC --> WS
    SOC --> CLI

    WS --> RTR
    CLI --> RTR

    RTR --> RH
    RTR --> UH
    RTR --> SH
    RTR --> CH
    RTR --> DH

    PC --> DC

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

    PC --> ICE
    PC --> SDP

    PC1 --> ICE
    PC2 --> ICE
    PC1 --> SDP
    PC2 --> SDP
```

## 開発

### 利用可能なコマンド

```bash
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
├── cmd/                    # アプリケーション
│   ├── signal/main.go      # シグナリングサーバーアプリ
│   └── datachannel/main.go # P2Pデータチャネルデモ
├── internal/               # 内部パッケージ（外部から非公開）
│   ├── webrtc/            # WebRTC関連コンポーネント
│   ├── transport/         # 通信・トランスポート層
│   └── protocol/          # プロトコル・メッセージ処理
├── domain/                # コアインターフェース・ドメインモデル
├── hub/                   # ハブ実装
├── signal/                # シグナリングサーバー
├── registry/              # ハンドラーレジストリ
├── logging/               # ログユーティリティ
│   ├── logger.go         # 構造化ログ実装
│   └── context.go        # ログコンテキスト
├── context.go             # コンテキストユーティリティ
├── go.mod                 # Goモジュール定義
└── Taskfile.yml           # タスクランナー設定
```

## Tips

ffmpegを使用して動画ファイルを変換するには、以下のコマンドを使用します：

```bash
ffmpeg -f rawvideo -vcodec rawvideo -s 640x480 -pix_fmt yuv420p -i bounce.yuv -c:v libx264 -pix_fmt yuv420p output.mp4
```

## 参考文献

### Pion WebRTCを使用したP2Pシグナリングサーバープロジェクト

#### ファイル転送系

- **[webwormhole](https://github.com/saljam/webwormhole)** - P2P認証付きWebRTC通信
- **[peerbeam](https://github.com/6b70/peerbeam)** - 高速・セキュアなP2Pファイル転送
- **[gfile](https://github.com/Antonito/gfile)** - WebRTC経由の直接ファイル転送

#### ネットワーク・トンネリング系

- **[pleer](https://github.com/raphadam/pleer)** - WebRTC経由でローカルポートをセキュアに転送
- **[rtctunnel](https://github.com/rtctunnel/rtctunnel)** - WebRTC上のネットワークトンネル
- **[ssh-p2p](https://github.com/nobonobo/ssh-p2p)** - SSH P2Pトンネリングサーバー/クライアント

#### 公式リソース

- **[Pion WebRTC Examples](https://github.com/pion/example-webrtc-applications)** - 実用的なWebRTCアプリケーション例
- **[Awesome Pion](https://github.com/pion/awesome-pion)** - Pionを使用したプロジェクト集
