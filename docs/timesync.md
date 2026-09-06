# 時刻同期（timesync）

PC の現在時刻を PLC の D デバイスへ書き込み、M デバイスの要求ビットを立てて PLC 側に時計の更新を依頼する機能です。PLC の内蔵時計を直接書き換えるのではなく、**PLC 側のラダープログラムに処理を委ねる**方式です。

## シーケンス

```
plccmd                                        PLC
  │
  │ 1. 時刻を D デバイスへ一括書き込み（7 ワード）
  ├──────────────────────────────────────────▶ D(yeard)   = 年
  │                                             D(yeard+1) = 月
  │                                             D(yeard+2) = 日
  │                                             D(yeard+3) = 時
  │                                             D(yeard+4) = 分
  │                                             D(yeard+5) = 秒
  │                                             D(yeard+6) = 曜日
  │
  │ 2. 要求ビットを ON
  ├──────────────────────────────────────────▶ M(reqm) = 1
  │                                                │
  │                                                │ ラダーが D の値で
  │                                                │ 内蔵時計を更新し、
  │                                                │ 結果ビットを立てて
  │                                                │ M(reqm) を OFF にする
  │                                                ▼
  │ 3. 200ms 周期で M(reqm) を含む 1 ワードを読み出し
  ├──────────────────────────────────────────▶
  ◀──────────────────────────────────────────┤ M(reqm)〜M(reqm+15)
  │
  │ 4. 要求ビットが OFF になったら完了
  ▼
```

## 実装（`internal/timesync/service.go`）

```go
func (s *Service) SyncNow(ctx context.Context, t time.Time, waitTimeout time.Duration) (*SyncResult, error)
```

処理は 4 段階です。

### 1. `validateTime`

年が 2000〜2099 の範囲外なら `year out of supported range: %d` を返して中断します。

### 2. `writeTime`

`time.Time` から 7 つの値を取り出し、D デバイスへ**一括書き込み**します。

| インデックス | 値 | 範囲 |
|---|---|---|
| 0 | `t.Year()` | 2000〜2099 |
| 1 | `t.Month()` | 1〜12 |
| 2 | `t.Day()` | 1〜31 |
| 3 | `t.Hour()` | 0〜23 |
| 4 | `t.Minute()` | 0〜59 |
| 5 | `t.Second()` | 0〜59 |
| 6 | `t.Weekday()` | 0=日曜 〜 6=土曜 |

書き込み先は `Devices.YearD` を先頭とした**連続 7 ワード**です。`MonthD` 以降のフィールドは参照されないため、PLC 側も連番のデバイスを割り当てる必要があります。

年は西暦 4 桁（例: `2026`）をそのまま 1 ワードに格納します。PLC 側で 2 桁年が必要な場合はラダーで変換してください。

### 3. `trigger`

`Devices.ReqM` の M デバイスに `0x0001` を 1 ワード書き込み、要求ビットを ON にします。

> ワード単位の書き込みのため、実際には `M(reqm)` 〜 `M(reqm+15)` の 16 点がまとめて更新されます。`M(reqm)` が ON、`M(reqm+1)` 〜 `M(reqm+15)` が OFF になります。結果ビットもこの範囲内にあるため、要求時に一緒にクリアされる形になります。

### 4. `waitCompletion`

200ms 間隔で `M(reqm)` から 1 ワード（16 点）を読み出し、ビットを判定します。

| ビット | 対応デバイス | 意味 |
|---|---|---|
| bit0 | `M(reqm)` | 同期要求 |
| bit2 | `M(reqm+2)` | 同期成功 |
| bit3 | `M(reqm+3)` | 同期エラー |

要求ビット（bit0）が OFF になった時点で完了と判断し、`SyncResult` を返します。`waitTimeout` を超えると `timeout waiting PLC to reset M100` を返します。`ctx` がキャンセルされた場合もその時点で中断します。

ビット位置は `ReqM+2` = 成功 / `ReqM+3` = エラーで確定しています（2026-09-05）。

## `SyncResult`

```go
type SyncResult struct {
    RequestTime time.Time  // 同期を要求した時刻
    CompletedAt time.Time  // 完了を検知した時刻
    Success     bool       // 成否
    ErrorFlag   bool       // エラービットの状態
}
```

`cmd/timesync.go` の `executeTimeSync` は `Success` が `false` の場合にエラーを返します。したがって `info: time sync success` が出力されるのは、**要求ビットが OFF になり、かつ成功ビット（`M(reqm+2)`）が ON、エラービット（`M(reqm+3)`）が OFF** の場合のみです。それ以外は `log.Panicf` により終了コード 2 で終了します。

## PLC 側ラダーの前提

このツールを使うには、PLC 側に次のようなプログラムが必要です。

1. `M(reqm)` の立ち上がりを検出する
2. `D(yeard)` 〜 `D(yeard+6)` の値を読み、内蔵時計（`DATEWR` 命令など）へ書き込む
3. 成功なら `M(reqm+2)`、失敗なら `M(reqm+3)` を ON にする
4. `M(reqm)` を OFF にする（これがツール側の完了検知トリガになる）

`waittimeout`（既定 10 秒）以内に手順 4 が実行されないと、ツール側はタイムアウトエラーになります。

## デバイス割り付け例

以下の割り付けの場合です。

| デバイス | 用途 |
|---|---|
| D100 | 年 |
| D101 | 月 |
| D102 | 日 |
| D103 | 時 |
| D104 | 分 |
| D105 | 秒 |
| D106 | 曜日 |
| M100 | 同期要求（ツールが ON、PLC が OFF） |
| M102 | 同期成功（PLC が ON） |
| M103 | 同期エラー（PLC が ON） |

## 想定される運用

常駐プロセスではなく、1 回実行して終了する CLI です。定期的に同期したい場合は cron（Linux/macOS）やタスクスケジューラ（Windows）から呼び出してください。ログをファイルに残す場合は `-logfile` を指定します（ローテーション設定は [architecture.md](./architecture.md) 参照）。
