# CLI リファレンス

## 書式

```
plccmd [グローバルフラグ] <サブコマンド> [サブコマンドのフラグ]
```

グローバルフラグはサブコマンド名より**前**に置く必要があります。

```sh
# OK
plccmd -loglevel=debug timesync -address=192.168.1.10:5002 ...

# NG（-loglevel が timesync のフラグとして解釈されエラーになる）
plccmd timesync -loglevel=debug -address=192.168.1.10:5002 ...
```

## グローバルフラグ

| フラグ | 既定値 | 説明 |
|---|---|---|
| `-loglevel` | `info` | 出力するログの最小レベル。`trace` / `debug` / `info` / `warning` / `error` / `alert` / `panic` |
| `-logfile` | （空） | 指定するとそのパスにもログを出力する。未指定なら stderr のみ |

## サブコマンド: `timesync`

PC の現在時刻を PLC のデバイスへ書き込み、時刻同期を要求します。処理の詳細は [timesync.md](./timesync.md) を参照してください。

### 接続関連フラグ

| フラグ | 既定値 | 必須 | 説明 |
|---|---|---|---|
| `-address` | （空） | ○ | PLC の宛先。`IP:ポート` 形式（例: `192.168.1.10:5002`） |
| `-dialtimeout` | `5` | | 接続タイムアウト（秒）※TCP 専用。UDP では未使用 |
| `-readtimeout` | `5` | | 受信タイムアウト（秒） |
| `-writetimeout` | `5` | | 送信タイムアウト（秒）※UDP では未使用 |
| `-waittimeout` | `10` | | 完了フラグのポーリング待ち時間（秒） |

### ルート情報フラグ

3E フレームのヘッダに載せる値です。既定値は自局アクセス向けの標準的な設定です。

| フラグ | 既定値 | 説明 |
|---|---|---|
| `-networkno` | `0` | ネットワーク番号 |
| `-pcno` | `255` | PC 番号（`0xFF`） |
| `-iono` | `1023` | 要求先ユニット I/O 番号（`0x03FF`） |
| `-stationno` | `0` | 要求先ユニット局番号 |
| `-monitortimer` | `16` | 監視タイマ（250ms 単位） |

### デバイス指定フラグ

すべて必須です。値はデバイス番号（10 進）を指定します。

| フラグ | デバイス | 説明 |
|---|---|---|
| `-yeard` | D | 年を書き込む先頭デバイス番号 |
| `-monthd` | D | 月 ※実際は `yeard` からの連番に書かれる |
| `-dayd` | D | 日 ※同上 |
| `-hourd` | D | 時 ※同上 |
| `-minuted` | D | 分 ※同上 |
| `-secondd` | D | 秒 ※同上 |
| `-wdayd` | D | 曜日（0=日曜〜6=土曜） ※同上 |
| `-reqm` | M | 同期要求ビット。`reqm+2` が成功、`reqm+3` がエラー |

> **注意**: これらのフラグは `validator` の `required` で検証されるため、**`0` を指定できません**。`D0` や `M0` を使う構成には対応していません。

### 環境変数

各フラグはフラグ名を大文字にした環境変数で既定値を上書きできます。優先順位は次のとおりです。

```
コマンドラインフラグ  >  環境変数  >  既定値
```

| 環境変数 | 対応フラグ |
|---|---|
| `ADDRESS` | `-address` |
| `DIALTIMEOUT` | `-dialtimeout` |
| `READTIMEOUT` | `-readtimeout` |
| `WRITETIMEOUT` | `-writetimeout` |
| `WAITTIMEOUT` | `-waittimeout` |
| `NETWORKNO` | `-networkno` |
| `PCNO` | `-pcno` |
| `IONO` | `-iono` |
| `STATIONNO` | `-stationno` |
| `MONITORTIMER` | `-monitortimer` |
| `YEARD` | `-yeard` |
| `MONTHD` | `-monthd` |
| `DAYD` | `-dayd` |
| `HOURD` | `-hourd` |
| `MINUTED` | `-minuted` |
| `SECONDD` | `-secondd` |
| `WDAYD` | `-wdayd` |
| `REQM` | `-reqm` |

環境変数の値が不正でパースに失敗した場合は `panic` します。

### 実行例

```sh
# フラグで指定
plccmd -loglevel=debug -logfile=./log/plccmd.log timesync \
  -address=192.168.1.10:5002 \
  -readtimeout=5 -waittimeout=10 \
  -yeard=100 -monthd=101 -dayd=102 \
  -hourd=103 -minuted=104 -secondd=105 -wdayd=106 \
  -reqm=100

# 環境変数で指定（タスクスケジューラ等から実行する場合に便利）
export ADDRESS=192.168.1.10:5002
export YEARD=100 MONTHD=101 DAYD=102 HOURD=103 MINUTED=104 SECONDD=105 WDAYD=106 REQM=100
plccmd timesync
```

### 終了時の挙動

- 成功時は `info: time sync success` を出力して正常終了します。成功と判定されるのは、要求ビットが OFF になり、かつ成功ビット `M(reqm+2)` が ON・エラービット `M(reqm+3)` が OFF の場合のみです。
- 失敗時（通信エラー、タイムアウト、PLC 側のエラー報告）は `cmd.Run` が error を返し、`app/main.go` の `log.Panicf` により panic して終了します（終了コードは 2）。

### 設定ファイル

`internal/config` に YAML 設定のローダがあり、`testdata/config.yaml` にサンプルがありますが、**CLI からは読み込まれていません**。現状の設定手段はフラグと環境変数のみです。

```yaml
plc:
  address: "192.168.1.10:5002"
  dial_timeout: "5s"
  read_timeout: "5s"
  write_timeout: "5s"
  wait_timeout: "10s"
  route:
    network_no: 0
    pc_no: 255
    io_no: 1023
    station_no: 0
    monitor_timer: 16
devices:
  year_d: 100
  month_d: 101
  day_d: 102
  hour_d: 103
  minute_d: 104
  second_d: 105
  wday_d: 106
  req_m: 100
```
