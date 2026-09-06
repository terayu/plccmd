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

3E フレームのヘッダに載せる値です。既定値は**自局の CPU にアクセスする**標準的な設定なので、PLC に直結する構成では変更不要です。

| フラグ | 既定値 | 意味 | 取りうる値 |
|---|---|---|---|
| `-networkno` | `0` | ネットワーク番号 | `0` = 自局ネットワーク。`1`〜`239` = MELSECNET/H や CC-Link IE 経由で他ネットワークへルーティングする場合のみ |
| `-pcno` | `255` | PC 番号（相手局） | `255`（`0xFF`）= 自局。`1`〜`64` = ネットワーク経由で他局を指定する場合 |
| `-iono` | `1023` | 要求先ユニット I/O 番号 | `1023`（`0x3FF`）= 自局 CPU。マルチ CPU 構成では下表を参照 |
| `-stationno` | `0` | 要求先ユニット局番号 | `0` = 通常。マルチドロップ接続で先の局を指定する場合のみ 0 以外 |
| `-monitortimer` | `16` | 監視タイマ | **250ms 単位**。`16` = 4 秒、`0` = 無限待ち |

`-iono` の代表値です。

| 値（10 進 / 16 進） | 対象 |
|---|---|
| `1023` / `0x3FF` | 自局 CPU（シングル CPU 構成はこれ） |
| `976` / `0x3D0` | 制御 CPU |
| `992`〜`995` / `0x3E0`〜`0x3E3` | マルチ CPU 構成の 1〜4 号機 |

> **移行時の注意**: `92a9dc4` 以前は、これらのフラグと環境変数が**すべて無視され**、`UDPClient` 内の固定値（`0` / `255` / `1023` / `0` / `16`）が使われていました。現在は指定した値が実際に送信されます。以前から `NETWORKNO` / `PCNO` / `IONO` / `STATIONNO` に別の値を設定していた場合、**その値が初めて有効になり PLC がエラー（例: 終了コード `0x4003`）を返します**。上表の既定値に戻すか、環境変数から削除してください。実際に送信される値は `-loglevel=debug` の `debug: iono:1023` などで確認できます。

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

値は 10 進のほか 16 進（`0x3FF`）や 8 進（`0777`）でも書けます（Go の `flag` が基数 0 でパースするため）。空文字列は無視されます。値が不正でパースに失敗した場合は `panic` します。

ルート情報を環境変数で指定する場合、自局アクセスなら次のとおりです。

```
NETWORKNO=0;PCNO=0xFF;IONO=0x3FF;STATIONNO=0;MONITORTIMER=16
```

いずれも既定値と同じなので、**設定しないのが最も確実**です。

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

設定ファイルには対応していません。設定手段はフラグと環境変数のみです。

以前は `internal/config` に未使用の YAML ローダがありましたが、CLI から参照されておらず、
唯一の `gopkg.in/yaml.v3` 依存でもあったため削除しました。

