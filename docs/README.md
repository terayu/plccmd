# plccmd ドキュメント

三菱電機系 PLC と **MC プロトコル（3E フレーム / バイナリ）** で通信する Go 製 CLI ツールです。
現在実装されているサブコマンドは `timesync`（PC の現在時刻を PLC のデバイスへ書き込み、時刻同期を要求する）のみです。

## 目次

| ドキュメント | 内容 |
|---|---|
| [architecture.md](./architecture.md) | パッケージ構成、依存関係、ログとビルドの仕組み |
| [cli.md](./cli.md) | コマンド・フラグ・環境変数のリファレンス |
| [timesync.md](./timesync.md) | 時刻同期のシーケンスと PLC 側ラダーの前提 |
| [mcprotocol.md](./mcprotocol.md) | **ライブラリとしての使い方**、デバイス一覧、3E フレームのバイト構造 |
| [known-issues.md](./known-issues.md) | 既知の問題・未使用コード |

## クイックスタート

```sh
# ビルド（go-task が必要。成果物は bin/plccmd）
task build

# Windows 向けクロスコンパイル（macOS ホストのみ）
task buildwin

# 実行
./bin/plccmd -loglevel=debug timesync \
  -address=192.168.1.10:5002 \
  -yeard=100 -monthd=101 -dayd=102 \
  -hourd=103 -minuted=104 -secondd=105 -wdayd=106 \
  -reqm=100
```

go-task を使わない場合は直接ビルドできます。

```sh
go build -tags timetzdata -o bin/plccmd app/*.go
```

## ライブラリとして使う

`mcprotocol` パッケージはモジュール直下の公開パッケージなので、別プロジェクトから import できます。

```go
import "github.com/terayu/plccmd/mcprotocol"

cfg := mcprotocol.DefaultConfig("192.168.1.10:5002")
c, err := mcprotocol.NewUDPClient(cfg)   // TCP なら NewTCPClient(cfg)
defer c.Close()

words, err := c.ReadWords(ctx, mcprotocol.DeviceD, 100, 7)
bits,  err := c.ReadBits(ctx, mcprotocol.DeviceM, 100, 4)
err = c.WriteWords(ctx, mcprotocol.DeviceZR, 0, []uint16{1, 2, 3})
err = c.WriteBits(ctx, mcprotocol.DeviceY, 0x20, []bool{true, false})
```

詳細は [mcprotocol.md](./mcprotocol.md) を参照してください。

タグがまだ無いため、`go get` すると擬似バージョン（`v0.0.0-<日付>-<ハッシュ>`）になります。開発中は `replace` か `go.work` でローカル参照するのが手軽です。

```
// 利用側の go.mod
require github.com/terayu/plccmd v0.0.0
replace github.com/terayu/plccmd => ../plccmd
```

`internal/timesync` と `internal/config` は `internal/` 配下のため、外部からは参照できません。

## テスト

```sh
go test ./...
```

| 対象 | 内容 |
|---|---|
| `mcprotocol` | フレーム構造、ビットデータの詰め替え、デバイス検証、点数上限、異常応答の扱い |
| `cmd`（E2E） | バイナリをビルドして擬似 PLC に対して実行し、終了コード・ログ・送信フレーム・書き込まれた値を検証 |

E2E テストはインプロセスではなく**サブプロセス**で実行します。`cmd` パッケージの `init()` が環境変数を読むこと、`flag.ExitOnError` がパースエラーでプロセスを終了させることから、環境変数とフラグの優先順位を検証するにはバイナリを起動する必要があるためです。テスト側は `ADDRESS` や `PCNO` などを意図的に引き継がないので、開発者のシェルの設定に影響されません。

### 実機に対するテスト

`PLCCMD_E2E_ADDRESS` を設定したときだけ実行される結合テストがあります。

```sh
PLCCMD_E2E_ADDRESS=192.168.1.10:5002 go test ./cmd/ -run RealPLC -v
```

| 環境変数 | 既定値 | 内容 |
|---|---|---|
| `PLCCMD_E2E_ADDRESS` | （必須） | PLC の `IP:ポート`。未設定ならスキップ |
| `PLCCMD_E2E_YEARD` | `1000` | 年を書き込む先頭 D デバイス番号 |
| `PLCCMD_E2E_REQM` | `10` | 同期要求の M デバイス番号 |

`TestRealPLCConnectivity` は読み出しのみで、到達性とデバイス割り付けを確認します。`TestRealPLCTimeSync` は timesync を実行したうえで、書き込まれた時刻と結果ビットを読み戻して検証します。

> **注意**: `TestRealPLCTimeSync` は D デバイスを上書きし、ラダーを起動して **PLC の内蔵時計を実際に更新します**。稼働中の設備には向けないでください。ルート情報は自局アクセス（`mcprotocol.DefaultConfig`）を前提としています。

## 動作要件

- Go 1.25.0 以上（`go.mod` 参照）
- PLC 側に Ethernet ユニット等の MC プロトコル（3E フレーム・バイナリ・UDP）通信設定
- PLC 側に時刻同期用のラダープログラム（[timesync.md](./timesync.md) 参照）

## 主な依存ライブラリ

| ライブラリ | 用途 |
|---|---|
| `github.com/comail/colog` | ログ出力（レベルはメッセージ接頭辞で指定） |
| `gopkg.in/natefinch/lumberjack.v2` | ログファイルのローテーション |
| `github.com/go-playground/validator/v10` | コマンド引数の必須チェック |
| `gopkg.in/yaml.v3` | YAML 設定の読み込み（現状未使用） |
