# アーキテクチャ

## ディレクトリ構成

```
plccmd/
├── app/
│   └── main.go              エントリポイント（ログ設定・サブコマンドのディスパッチ）
├── cmd/
│   └── timesync.go          timesync サブコマンドの引数定義と実行
├── mcprotocol/              MC プロトコル 3E フレームの通信層（公開パッケージ）
│   ├── types.go             Client インターフェース、Config、EndCodeError
│   ├── device.go            デバイスコード定義
│   ├── frame3e.go           フレーム組み立て・解析、ビットデータの詰め替え
│   ├── client.go            読み書き 4 メソッドの共通実装
│   ├── tcpClient.go         TCP トランスポート
│   └── udpClient.go         UDP トランスポート（timesync が使う）
├── internal/
│   └── timesync/            時刻同期の業務ロジック
│       ├── types.go         Devices / SyncResult / WordReaderWriter
│       └── service.go       SyncNow（書き込み→トリガ→完了待ち）
├── bin/                     ビルド成果物
├── Taskfile.yml             OS 別 Taskfile の取り込み
├── Taskfile_darwin.yml      macOS 用ビルドタスク（build / buildwin）
└── Taskfile_windows.yml     Windows 用ビルドタスク（build）
```

## レイヤ構造

```
app/main.go
    │  グローバルフラグを解析し、残り引数を map からサブコマンドへ渡す
    ▼
cmd/timesync.go
    │  timesync 専用 FlagSet を解析 → validator で必須チェック
    │  mcprotocol クライアントを生成し timesync.Service に注入
    ▼
internal/timesync/Service
    │  timesync.WordReaderWriter インターフェース越しに読み書き
    ▼
mcprotocol/UDPClient
       3E フレームを組み立て、ソケットで送受信
```

`timesync.Service` は具体的なクライアント型ではなく、自分が必要とする 2 メソッドだけを切り出したインターフェースに依存しています。

```go
// internal/timesync/types.go
type WordReaderWriter interface {
    ReadWords(ctx context.Context, dev mcprotocol.DeviceCode, head uint32, points uint16) ([]uint16, error)
    WriteWords(ctx context.Context, dev mcprotocol.DeviceCode, head uint32, values []uint16) error
}
```

`mcprotocol.Client`（`*UDPClient` / `*TCPClient`）がこれを満たすため、通信方式の差し替えやテスト用モックの注入が可能です。

### `mcprotocol` パッケージの内部構造

読み書きの 4 メソッドはトランスポートに依存しないため、`client` 構造体に一度だけ実装し、各クライアントが埋め込んでいます。

```
UDPClient / TCPClient
  ├─ client（埋め込み）      ReadWords / WriteWords / ReadBits / WriteBits
  │    └─ tr transport       exchange(ctx, frame) ([]byte, error)
  └─ exchange()              トランスポート固有の送受信
```

フレームの組み立てと解析（`buildRequest` / `parseResponse` / `packBits` / `unpackBits`）は `frame3e.go` に集約されています。

## サブコマンドのディスパッチ

`app/main.go` はサブコマンド名から実行関数を引く単純なマップを持っています。

```go
fs := make(map[string]func([]string) error)
fs["timesync"] = cmd.TimeSyncCommand.Run
```

グローバルフラグ（`-loglevel` / `-logfile`）は `flag.Parse()` が処理し、`flag.Args()` の先頭がサブコマンド名、残りがサブコマンドの引数として渡されます。そのためグローバルフラグは**サブコマンド名より前**に置く必要があります。

未知のサブコマンド名、およびサブコマンドがエラーを返した場合は `log.Panicf` で終了します。

## ログ

`colog` + `lumberjack` の組み合わせです。

- 出力先は常に `os.Stderr`。`-logfile` を指定すると `io.MultiWriter` でファイルにも同時出力します。
- ログレベルはメッセージの接頭辞で表現します（`log.Printf("debug: ...")` / `"info: "` / `"trace: "` / `"err: "` など）。`colog` がこの接頭辞を解釈し、`-loglevel` 未満のものを抑制します。
- フォーマットは `log.Ldate | log.Ltime | log.Llongfile`、カラー出力は無効。
- ローテーション設定（`app/main.go`）: 最大 10MB、保持 60 日、バックアップ 10 世代、gzip 圧縮、タイムスタンプは UTC。

## ビルド

`Taskfile.yml` が `Taskfile_{{OS}}.yml` を `flatten` で取り込むため、OS ごとに定義が切り替わります。

| タスク | 定義場所 | 内容 |
|---|---|---|
| `build` | darwin / windows | `go build -tags timetzdata -o bin/plccmd[.exe] app/*.go` |
| `buildwin` | darwin のみ | `GOOS=windows GOARCH=amd64` でクロスコンパイル |

`-tags timetzdata` により tzdata をバイナリへ埋め込み、tzdata の無い環境でもタイムゾーン解決ができるようにしています。

`app/main.go` には `version` / `buildTime` というリンカ埋め込み用の変数がありますが、Taskfile 側に `-ldflags` の指定がないため現状は常に `unknown` が出力されます（[known-issues.md](./known-issues.md) 参照）。
