// Package mcprotocol は三菱電機系 PLC と MC プロトコルで通信するクライアントである。
//
// 対応するのは 3E フレーム・バイナリコードのみで、トランスポートは UDP と TCP を
// 選べる。Go 標準ライブラリ以外への依存は無い。
//
// # 使い方
//
// [DefaultConfig] を起点に設定を組み立て、[NewUDPClient] か [NewTCPClient] で
// 接続する。どちらも [Client] を実装するので、呼び出し側はトランスポートを
// 意識せずに書ける。
//
//	cfg := mcprotocol.DefaultConfig("192.168.1.10:5002")
//
//	c, err := mcprotocol.NewUDPClient(ctx, cfg) // TCP なら NewTCPClient(ctx, cfg)
//	if err != nil {
//		return err
//	}
//	defer c.Close()
//
//	words, err := c.ReadWords(ctx, mcprotocol.DeviceD, 100, 7) // D100〜D106
//	err = c.WriteBits(ctx, mcprotocol.DeviceM, 100, []bool{true})
//
// ゼロ値の [Config] をそのまま渡すと PC 番号と I/O 番号が 0 になり PLC に
// 拒否される。[DefaultConfig] の戻り値を必要な項目だけ変更して使うこと。
//
// # デバイス番号の指定
//
// head にはデバイス番号を 10 進の値として渡す。X/Y/B/W/ZR のように三菱の表記が
// 16 進のデバイスは、変換後の値が必要になる（X10 なら 16。Go のリテラルとしては
// 0x10 と書ける）。
//
// # ctx の効き方
//
// [NewUDPClient] と [NewTCPClient] は接続の確立を ctx でキャンセルできる。
//
// 読み書きの各メソッドでは、ctx は締切の算出にのみ使う。[Config] のタイムアウトと
// ctx の期限のうち早いほうが締切になる。送信前にキャンセル済みかは確認するが、
// I/O の実行中に割り込むことはしない。
//
// # エラー
//
// PLC が 0 以外の終了コードを返した場合は [*EndCodeError] になる。それ以外
// （接続失敗、タイムアウト、フレーム不正、引数不正）は通常の error である。
//
//	var ece *mcprotocol.EndCodeError
//	if errors.As(err, &ece) {
//		log.Printf("PLC error 0x%04X", ece.Code)
//	}
//
// # UDP を選ぶときの注意
//
// UDP には要求と応答を対応付ける仕組みが無く、3E フレームにも 4E のような
// シリアル番号が無い。応答が ReadTimeout を超えて遅延すると、以降は 1 つずれた
// 応答を読み続けることになる。確実性が必要な場合は [NewTCPClient] を使うこと。
package mcprotocol
