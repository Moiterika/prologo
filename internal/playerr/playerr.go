// Package playerr は「プレイヤーの入力がルール上おかしい」ことを表す誤りを
// 1 つだけ提供する。
//
// 3 つのゲームがどれも同じ扱いを必要とする。サーバーはこの誤りを 500 では
// なく画面上のメッセージとして出すので、内部の不具合 (ルールファイルの誤り
// など) と区別できるように型を分けてある。
package playerr

import "fmt"

// Error はプレイヤーへ見せるメッセージを持つ誤り。
type Error struct{ Msg string }

func (e Error) Error() string { return e.Msg }

// Errorf は書式付きで Error を作る。
func Errorf(format string, a ...interface{}) error {
	return Error{Msg: fmt.Sprintf(format, a...)}
}
