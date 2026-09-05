# mcprotocol パッケージ

`github.com/terayu/plccmd/mcprotocol` は MC プロトコル 3E フレーム・**バイナリコード**のクライアントです。モジュール直下の公開パッケージなので、別プロジェクトからも import できます。

```go
import "github.com/terayu/plccmd/mcprotocol"
```

## 使い方

```go
cfg := mcprotocol.DefaultConfig("192.168.1.10:5002")

var c mcprotocol.Client
c, err := mcprotocol.NewUDPClient(cfg)   // TCP なら NewTCPClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

ctx := context.Background()

words, err := c.ReadWords(ctx, mcprotocol.DeviceD, 100, 7)   // D100〜D106
bits,  err := c.ReadBits(ctx, mcprotocol.DeviceM, 100, 4)    // M100〜M103
err = c.WriteWords(ctx, mcprotocol.DeviceZR, 0, []uint16{1, 2, 3})
err = c.WriteBits(ctx, mcprotocol.DeviceY, 0x20, []bool{true, false})
```

## `Client` インターフェース

`*UDPClient` と `*TCPClient` の両方が実装します。

```go
type Client interface {
    ReadWords(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]uint16, error)
    WriteWords(ctx context.Context, dev DeviceCode, head uint32, values []uint16) error
    ReadBits(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]bool, error)
    WriteBits(ctx context.Context, dev DeviceCode, head uint32, values []bool) error
    Close() error
}
```

`head` は**デバイス番号をそのまま 10 進で**渡します。X/Y/B/W/ZR など三菱の表記が 16 進のデバイスは、変換後の値が必要です（`X10` なら `16`、Go のリテラルなら `0x10` と書けます）。

`ctx` は期限の算出に使われ、`Config` のタイムアウトと ctx の期限のうち**早いほう**が締切になります。送信前にキャンセル済みかを確認しますが、**I/O 実行中の割り込みはしません**（打ち切りは締切による）。

## `Config`

```go
type Config struct {
    Address      string        // "IP:ポート"
    DialTimeout  time.Duration // TCP のみ
    ReadTimeout  time.Duration
    WriteTimeout time.Duration

    NetworkNo    byte
    PCNo         byte
    IONo         uint16
    StationNo    byte
    MonitorTimer uint16        // 250ms 単位
}
```

`DefaultConfig(address)` が自局アクセス向けの標準値（ネットワーク番号 `0x00` / PC 番号 `0xFF` / I/O 番号 `0x03FF` / 局番号 `0x00` / 監視タイマ `0x0010`、各タイムアウト 5 秒）を返します。ゼロ値の `Config` をそのまま渡すと PC 番号・I/O 番号が `0` になり PLC に拒否されるため、`DefaultConfig` を起点に必要な項目だけ変更してください。

## デバイスコード

`DeviceCode` 型で表します。`String()` で名前が、`IsBit()` でビットアクセスの可否が得られます。

### ビットデバイス（ビット単位・ワード単位の両方でアクセス可能）

| 定数 | 値 | デバイス |
|---|---|---|
| `DeviceX` | `0x9C` | 入力 |
| `DeviceY` | `0x9D` | 出力 |
| `DeviceM` | `0x90` | 内部リレー |
| `DeviceL` | `0x92` | ラッチリレー |
| `DeviceB` | `0xA0` | リンクリレー |
| `DeviceTS` | `0xC1` | タイマ接点 |
| `DeviceTC` | `0xC0` | タイマコイル |
| `DeviceCS` | `0xC4` | カウンタ接点 |
| `DeviceCC` | `0xC3` | カウンタコイル |

ワード単位でアクセスすると 1 ワード = 16 点として扱われます。

### ワードデバイス（ワード単位のみ）

| 定数 | 値 | デバイス |
|---|---|---|
| `DeviceD` | `0xA8` | データレジスタ |
| `DeviceW` | `0xB4` | リンクレジスタ |
| `DeviceR` | `0xAF` | ファイルレジスタ |
| `DeviceZR` | `0xB0` | ファイルレジスタ(連番アクセス) |
| `DeviceTN` | `0xC2` | タイマ現在値 |
| `DeviceCN` | `0xC5` | カウンタ現在値 |

ワードデバイスに `ReadBits` / `WriteBits` を呼ぶと、通信する前にエラーになります。

> T と C はデバイスとして 3 つの実体（接点・コイル・現在値）を持つため、`DeviceTS` / `DeviceTC` / `DeviceTN`、`DeviceCS` / `DeviceCC` / `DeviceCN` に分かれています。

## エラー

PLC が 0 以外の終了コードを返した場合は `*EndCodeError` になります。

```go
var ece *mcprotocol.EndCodeError
if errors.As(err, &ece) {
    log.Printf("PLC error 0x%04X", ece.Code)
}
```

それ以外（接続失敗、タイムアウト、フレーム不正、引数不正）は通常の error です。

## 点数の上限

| アクセス | 上限 |
|---|---|
| ワード単位 | 960 点（`MaxWordPoints`） |
| ビット単位 | 3584 点（`MaxBitPoints`） |

上限超過と 0 点は、通信する前にエラーになります。

## フレーム構造

### 要求フレーム

ヘッダは 11 バイトです。数値はすべてリトルエンディアンです。

| オフセット | 長さ | 内容 |
|---|---|---|
| 0 | 2 | サブヘッダ `0x50 0x00` |
| 2 | 1 | ネットワーク番号 |
| 3 | 1 | PC 番号 |
| 4 | 2 | 要求先ユニット I/O 番号 |
| 6 | 1 | 要求先ユニット局番号 |
| 7 | 2 | 要求データ長 = `2 + len(要求データ)` |
| 9 | 2 | 監視タイマ |
| 11 | n | 要求データ |

### 要求データ

| オフセット | 長さ | 内容 |
|---|---|---|
| 0 | 2 | コマンド（`CommandBatchRead` = `0x0401` / `CommandBatchWrite` = `0x1401`） |
| 2 | 2 | サブコマンド（`SubcommandWord` = `0x0000` / `SubcommandBit` = `0x0001`） |
| 4 | 3 | 先頭デバイス番号 |
| 7 | 1 | デバイスコード |
| 8 | 2 | デバイス点数 |
| 10 | — | 書き込みデータ（書き込み時のみ） |

ビット単位かワード単位かは**サブコマンドだけ**で決まり、コマンドは共通です。

### 応答フレーム

| オフセット | 長さ | 内容 |
|---|---|---|
| 0 | 2 | サブヘッダ `0xD0 0x00` |
| 2 | 5 | ルート情報 |
| 7 | 2 | 応答データ長 |
| 9 | 2 | 終了コード（`0x0000` が正常） |
| 11 | n | 応答データ（読み出し時のみ） |

### ビットデータの詰め方

ビット単位では **1 バイトに 2 点**が詰められます。上位 4 ビットが先頭の点、下位 4 ビットが次の点です。点数が奇数のとき、書き込み要求では末尾の下位 4 ビットが 0 になります。

```
[true, false, true] → 0x10 0x10
                        │ │  └── 3点目 = true（上位ニブル）
                        │ └───── 2点目 = false（下位ニブル）
                        └─────── 1点目 = true（上位ニブル）
```

データ部の長さは `(点数+1)/2` バイトです。

## UDP と TCP の違い

| | `UDPClient` | `TCPClient` |
|---|---|---|
| 応答サブヘッダの検証 | ○ | ○ |
| 応答長・データ長の検証 | ○ | ○ |
| 点数の範囲チェック | ○ | ○ |
| `DialTimeout` | 未使用（`DialUDP` はハンドシェイクしない） | 使用 |
| 要求と応答の対応付け | **不可** | 可 |

3E フレームには 4E のようなシリアル番号が無いため、UDP では応答が `ReadTimeout` を超えて遅延すると、以降ずっと 1 つずれた応答を読み続けます。確実性が必要な用途では `TCPClient` を使ってください。

## 未対応

- ランダム読み書き、モニタ登録などの一括読み書き以外のコマンド
- ASCII コード、4E / 1E フレーム
- 上表以外のデバイス（SM/SD/F/V/S/Z/SB/SW/DX/DY など）
