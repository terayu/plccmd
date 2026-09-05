# 既知の問題・未使用コード

コードリーディングで見つかった、動作に影響しうる点および整理候補をまとめたものです。いずれも 2026-09-04 時点（コミット `92a9dc4`）の内容で、まだ修正はしていません。

## 優先度

| 優先度 | 項目 | 影響 |
|---|---|---|
| ~~最高~~ 済 | [1. PLC 側のエラーが握りつぶされる](#1-plc-側のエラーが握りつぶされる-対応済み) | **対応済み**（2026-09-05）。`result.Success` を検査してエラーを返すようにした |
| ~~最高~~ 済 | [4. 完了ビットの 1 ビットずれ](#4-完了ビットの判定とコメントが食い違う-対応済み) | **対応済み**（2026-09-05）。コメント・ヘルプ文言とも実装に一致 |
| ~~最高~~ 済 | [5. `Success` の判定式](#5-success-の判定式が実質機能していない-対応済み) | **対応済み**（2026-09-05）。`successFlag && !errFlag` に修正 |
| ~~高~~ 済 | [3. 接続関連フラグが反映されない](#3-接続関連フラグが反映されない-対応済み) | **対応済み**（2026-09-05）。`NewUDPClient` が `Config` を受け取るようにした |
| ~~高~~ 済 | [6. UDP 応答長チェックなし](#6-udp-クライアントに応答長のチェックがない-対応済み) | **対応済み**（2026-09-05）。長さ・サブヘッダを検証し、cap を切り詰めた |
| 中 | [9. `writeTime` は `YearD` 以外を使わない](#9-writetime-は-yeard-以外のデバイス番号を使わない) | 非連番の割り付けにすると無関係な D デバイス 7 点を破壊する |
| 低 | [2](#2-ログファイルの絶対パス出力が壊れている) / [7](#7-デバイス番号に-0-を指定できない) / [8](#8-version--buildtime-が常に-unknown) | ログ表示・設定上の制約のみで実害は限定的 |

優先度「最高」だった 3 件（1・4・5）は 2026-09-05 に対応済みです。ビット位置は `ReqM+2` = 成功 / `ReqM+3` = エラーで確定しました。

> **要実機確認**: 項目 5 の修正により、**PLC が `M(reqm+2)` を ON にしない限り成功と判定されなくなりました**。ラダーが成功ビットを立てない実装だと、以前は成功扱いだったケースが失敗になります。実機での動作確認が必要です。

未対応で残っているのは項目 7・8・9・10 です。

## 不具合

### 1. PLC 側のエラーが握りつぶされる（対応済み）

**2026-09-05 対応**: `cmd/timesync.go` の `executeTimeSync` で `result.Success` を検査し、失敗時はエラーを返すようにしました。

```go
	result, err := service.SyncNow(context.Background(), now, waitTimeout)
	if err != nil {
		log.Printf("err: time sync failed result:%+v", result)
		return
	}
	if !result.Success {
		err = fmt.Errorf("plc reported time sync failure: result:%+v", result)
		log.Printf("err: %v", err)
		return
	}
```

修正前は `SyncNow` が要求ビットの OFF を検知しさえすれば `err = nil` を返し、`SyncResult.Success` が `false` でも `info: time sync success` を出力して**終了コード 0 で正常終了**していました。現在は `Run` へエラーが伝播し、`main` の `log.Panicf` により終了コード 2 で終わります。

### 2. ログファイルの絶対パス出力が壊れている

`app/main.go:65`

```go
_, abs := filepath.Abs(logfile)
log.Printf("info: logfile: %s\n", abs)
```

`filepath.Abs` は `(string, error)` を返すため、`abs` に入るのは絶対パスではなく **error** です。結果として `info: logfile: %!s(<nil>)` が出力されます。正しくは `abs, _ := filepath.Abs(logfile)`。

### 3. 接続関連フラグが反映されない（対応済み）

**2026-09-05 対応**: `NewUDPClient` が `Config` を受け取るようになり、`-networkno` / `-pcno` / `-iono` / `-stationno` / `-monitortimer` が反映されるようになりました。

```go
client, err := mcprotocol.NewUDPClient(mcCfg)
```

修正前は `mcprotocol.Config` を組み立てても `NewUDPClient(mcCfg.Address, mcCfg.ReadTimeout)` としか呼んでおらず、`UDPClient` のコンストラクタ内固定値が使われていました。

> **既定値を変更しています**: フラグが実際に反映されるようになったため、これまでの実効値を維持するよう `-pcno` の既定値を `0` → `255`、`-iono` を `0` → `1023` に変更しました。
>
> **既存環境での影響（2026-09-05 に実機で発生・解決済み）**: 既定値の調整だけでは不十分で、**環境変数 `NETWORKNO` / `PCNO` / `IONO` / `STATIONNO` に値を設定していた場合、その値が初めて有効になります**。実機では `NETWORKNO=2` `PCNO=0` `IONO=255` `STATIONNO=2` が設定されており、ルート情報 5 バイトすべてが自局アクセスの `00 FF FF 03 00` から `02 00 FF 00 02` に変わって PLC が終了コード `0x4003` を返しました。これらの環境変数を削除して解決しています。移行時は `-loglevel=debug` で実際の値を確認してください。

`-dialtimeout` は TCP 専用です（`net.DialUDP` はハンドシェイクを行わないため UDP では不要）。`-writetimeout` も UDP では `SetDeadline` が読み書き共通のため使われません。ヘルプ文言にその旨を記載しています。

### 4. 完了ビットの判定とコメントが食い違う（対応済み）

**2026-09-05 対応**: `internal/timesync/service.go:79-80` のコメントを実装（`bit2` = `ReqM+2` 成功 / `bit3` = `ReqM+3` エラー）に合わせて修正済み。ビット位置は `ReqM+2` / `ReqM+3` が正として確定しました。

```go
// s.devices.ReqM is request sync
// s.devices.ReqM+2 is success time sync
// s.devices.ReqM+3 is error time sync
requestFlag := (w & (1 << 0)) != 0
successFlag := (w & (1 << 2)) != 0
errFlag     := (w & (1 << 3)) != 0
```

`cmd/timesync.go:59` の `-reqm` フラグのヘルプ文言も `(reqm+2 is success, reqm+3 is error)` に修正済みで、コメント・ヘルプ・実装の 3 つが一致しています。

### 5. `Success` の判定式が実質機能していない（対応済み）

**2026-09-05 対応**: `internal/timesync/service.go:90` を修正しました。

```go
Success: successFlag && !errFlag,
```

修正前は `!errFlag && (successFlag || !errFlag)` で、`!errFlag` が真なら括弧の中も必ず真になるため実質 `!errFlag` と等価でした。`successFlag` が結果に影響していませんでした。

> この修正により、**PLC が `M(reqm+2)` を ON にしない限り成功と判定されません**。ラダーが成功ビットを立てない実装の場合は失敗扱いになるため、実機での確認が必要です。

### 6. UDP クライアントに応答長のチェックがない（対応済み）

**2026-09-05 対応**: `sendAndReceive` に長さとサブヘッダの検証を追加し、戻り値の cap を受信バイト数に切り詰めました。`BatchReadWords` にはデータ長のチェックを追加しています。

```go
	if n < 11 {
		return nil, fmt.Errorf("response too short: %d", n)
	}
	if buf[0] != 0xD0 || buf[1] != 0x00 {
		return nil, fmt.Errorf("unexpected response subheader: %02X %02X", buf[0], buf[1])
	}

	// cap を n に切り詰め、以降のスライスで未受信領域を読まないようにする
	return buf[:n:n], nil
```

```go
	data := resp[11:]
	if len(data) != int(points)*2 {
		return nil, fmt.Errorf("unexpected data length: got %d want %d", len(data), int(points)*2)
	}
```

検証用に `internal/mcprotocol/udpClient_test.go` を追加しました（ダミー UDP サーバに各種の異常応答を返させる 6 ケース）。

なお **UDP の要求／応答が対応付けられない構造的な制約は残っています**（下記「発生条件」の 2 点目）。これを解消するには `TcpClient` への切り替えが必要です（項目 3 と関連）。

以下は修正前の分析です。

`sendAndReceive` は受信バイト数を検証せずに返します。

```go
buf := make([]byte, 2048)
n, err := c.conn.Read(buf)
...
return buf[:n], nil     // len=n, cap=2048
```

戻り値の **cap が 2048 のまま**である点が重要です。Go のスライス式は上限を len ではなく **cap** で境界チェックするため、`resp[9:11]` は n がいくつであっても通過し、`make` がゼロ初期化した未使用領域を読みます。

以下は実測値です（n=5 の応答を模擬）。

```
resp: len=5 cap=2048

resp[9:11] (endCode)         no panic   -> endCode = 0x0000
resp[11:] (data)             PANIC: slice bounds out of range [11:5]
data[i*2:] (n=11, points=7)  PANIC: index out of range [1] with length 0
```

#### A. `BatchWriteWords` — 不正応答を成功と誤認する（`udpClient.go:105`）

11 バイト未満の応答では終了コードとしてゼロ埋め領域を読むため `endCode == 0` となり、**`return nil`（成功）を返します**。panic せず静かに誤るため、B・C より影響が大きい経路です。`writeTime` と `trigger` の両方がこの関数を使うため、時刻が PLC に書かれていないまま次の段階へ進みます。

#### B. `BatchReadWords` — 短い応答で panic（`udpClient.go:135`）

`resp[11:]` は上限が `len(resp)` に既定されるため `11 <= n` が必要で、n が 11 未満だと panic します。A と挙動が分かれるのは、`[low:]` 形式の下限チェックが len に対して行われるためです。

#### C. `BatchReadWords` — データ長不足で panic（`udpClient.go:139`）

`len(data)` と `points*2` を突き合わせていないため、要求点数に対して応答が短いとループ内の `binary.LittleEndian.Uint16` が panic します。

#### `TcpClient` との比較

| 検証 | `TcpClient` | `UDPClient`（修正前） | `UDPClient`（修正後） |
|---|---|---|---|
| 応答サブヘッダが `0xD0 0x00` か | ○ | ✗ | ○ |
| `len(resp) >= 11`（終了コード読み出し前） | ○ | ✗ | ○ |
| `len(data) == points*2` | ○ | ✗ | ○ |
| 点数が 1〜960 の範囲か | ○ | ✗ | ✗ |

#### 発生条件

`net.DialUDP` は接続済みソケットになるため、宛先以外の IP からのデータグラムは弾かれます。現実的な経路は次の 2 つです。

- PLC 側が想定外の短い応答・不正なフレームを返す
- **UDP の応答が遅延し、次の要求の応答として拾われる** — 3E フレームには 4E のようなシリアル番号が無く、このコードも要求と応答を対応付けていません。`waitCompletion` は 200ms 周期でポーリングするため、`-readtimeout` を超えて遅延した応答が 1 つ出ると、以降ずっと 1 つずれた応答を読み続けます

後者は項目 6 単体ではなく **UDP を使ううえでの構造的な制約**で、`TcpClient` に切り替えれば同時に解消します（項目 3 と関連）。

#### 修正方針

`sendAndReceive` で長さとサブヘッダを確定させるのが最小の修正です。

```go
if n < 11 {
    return nil, fmt.Errorf("response too short: %d", n)
}
if buf[0] != 0xD0 || buf[1] != 0x00 {
    return nil, fmt.Errorf("unexpected response subheader: %02X %02X", buf[0], buf[1])
}
return buf[:n:n], nil   // cap を n に切り詰め、cap 経由の読み出しを構造的に防ぐ
```

3 インデックススライス `buf[:n:n]` にしておくと A の経路を塞げます。加えて `BatchReadWords` に `len(data) == int(points)*2` のチェックを入れれば C も塞がります。

### 7. デバイス番号に 0 を指定できない

`cmd/timesync.go` のデバイス指定フィールドはすべて `validate:"required"` です。`validator` は数値の `required` を「ゼロ値でないこと」として扱うため、`-yeard=0` や `-reqm=0` は検証エラーになります。`D0` / `M0` を使う割り付けには対応できません。

### 8. `version` / `buildTime` が常に `unknown`

`app/main.go` の `version` / `buildTime` はリンカで埋め込む前提の変数ですが、`Taskfile_darwin.yml` / `Taskfile_windows.yml` のビルドコマンドに `-ldflags` の指定がありません。そのため `debug: version: unknown, buildTime: unknown` が出力され続けます。

## 設計上の注意点

### 9. `writeTime` は `YearD` 以外のデバイス番号を使わない

`internal/timesync/service.go:51`

```go
return s.io.BatchWriteWords(mcprotocol.DeviceCodeD, s.devices.YearD, values)
```

`YearD` からの連続 7 ワードへ書き込むため、`MonthD` 〜 `WdayD` の指定値は無視されます。フラグ上は必須なのに実際は使われないため、連番以外を指定すると期待と異なる結果になります。

### 10. `trigger` がワード書き込みで隣接ビットも書き換える

`M(reqm)` に対して 1 ワード（16 点）を書き込むため、`M(reqm+1)` 〜 `M(reqm+15)` も同時に OFF になります。結果ビット（`reqm+2` / `reqm+3`）が同じワード内にあるため意図した挙動と思われますが、この範囲を他用途に使うことはできません。

## 未使用コード

削除ではなく、今後の利用予定の有無を確認したい箇所です。

| 対象 | 状況 |
|---|---|
| `internal/config` パッケージ | `Load` / `Validate` / `Must*Timeout` すべて未参照。CLI は設定ファイルを読まない |
| `testdata/config.yaml` | 上記に対応するサンプル。参照元なし |
| `mcprotocol.TCPClient` | CLI からは未使用（公開パッケージなので外部利用は可能）。UDP と違い要求／応答を対応付けられる |

## 軽微な整理候補

- `internal/timesync` にテストがありません。`WordReaderWriter` インターフェースがあるので、モックを差し込めばテストしやすい構造です（`mcprotocol` にはテストがあります）。
- `.gitignore` に `bin/` の記載がなく、ビルド成果物 `bin/plccmd` が未追跡ファイルとして残ります（`*.exe` は除外されるため `bin/plccmd.exe` のみ無視される）。同様に `go.sum` も未コミットです。**別プロジェクトから利用する場合、`go.sum` は利用側の障害にはなりません**（`go.sum` はメインモジュールでのみ参照されるため）が、リポジトリの再現性のためコミットすべきです。
