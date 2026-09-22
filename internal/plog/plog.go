// Package plog は Prolog 処理系 (github.com/ichiban/prolog) を包む薄い層。
//
// ルールファイルの読み込みと、Go からの問い合わせだけを受け持つ。
// ゲームのルールそのものは一切知らない。
//
// ichiban/prolog は ISO 準拠の純 Go 実装なので、SWI-Prolog 固有の述語
// (include/3, flatten/2, forall/2 など) や `:- dynamic` の前置演算子が無い。
// そこを埋める compat.pl をこのパッケージが持っていて、どのゲームの
// ルールファイルより先に必ず読み込む。
package plog

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ichiban/prolog"
)

//go:embed compat.pl
var compatFS embed.FS

// ErrNoSolution は問い合わせに解が無かったことを表す。
// 「失敗」は Prolog では正常な結果なので、Prove はこれを false に読み替える。
var ErrNoSolution = errors.New("解がありません")

// Interp は 1 つの Prolog インタプリタ。
//
// 並行安全ではない。assertz/retract でルール設定を書き換えるため、
// 対局ごとに 1 つ作ること。
type Interp struct {
	p *prolog.Interpreter
}

// New は Prolog を起動し、compat.pl に続けて files を読み込む。
//
// dir が空なら fsys (呼び出し側が go:embed したもの) から、そうでなければ
// そのディレクトリの .pl を読む。後者は再ビルドせずにルールを書き換えて
// 試すためのもの。
func New(fsys embed.FS, dir string, files []string) (*Interp, error) {
	// Prolog 側に標準入力は与えない (端末入力は Go の UI が独占する)。
	p := prolog.New(strings.NewReader(""), os.Stdout)

	src, err := compatFS.ReadFile("compat.pl")
	if err != nil {
		return nil, fmt.Errorf("compat.pl の読み込みに失敗しました: %w", err)
	}
	if err := p.Exec(string(src)); err != nil {
		return nil, fmt.Errorf("compat.pl の解釈に失敗しました: %w", err)
	}

	for _, name := range files {
		var src []byte
		var err error
		if dir == "" {
			src, err = fsys.ReadFile(name)
		} else {
			src, err = os.ReadFile(filepath.Join(dir, name))
		}
		if err != nil {
			return nil, fmt.Errorf("%s の読み込みに失敗しました: %w", name, err)
		}
		if err := p.Exec(string(src)); err != nil {
			return nil, fmt.Errorf("%s の解釈に失敗しました: %w", name, err)
		}
	}
	return &Interp{p: p}, nil
}

// Scan は最初の解を dest に取り込む。解が無い場合は ErrNoSolution を返す。
// dest が nil なら成否だけを見る。
func (i *Interp) Scan(query string, dest interface{}) error {
	sol := i.p.QuerySolution(query)
	if err := sol.Err(); err != nil {
		if errors.Is(err, prolog.ErrNoSolutions) {
			return ErrNoSolution
		}
		return fmt.Errorf("問い合わせ %s に失敗しました: %w", query, err)
	}
	if dest == nil {
		return nil
	}
	if err := sol.Scan(dest); err != nil {
		return fmt.Errorf("問い合わせ %s の結果を読み取れませんでした: %w", query, err)
	}
	return nil
}

// Prove はゴールが成功するかどうかだけを判定する。
func (i *Interp) Prove(query string) (bool, error) {
	err := i.Scan(query, nil)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrNoSolution):
		return false, nil
	default:
		return false, err
	}
}

// Set は「設定を切り替える」述語を呼び、失敗したら分かりやすい誤りにする。
// 画面から来た値をそのまま渡しても、不正なら Prolog 側の member/2 で弾かれる。
func (i *Interp) Set(query, what string) error {
	ok, err := i.Prove(query)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s の指定が不正です: %s", what, query)
	}
	return nil
}

// =============================================
// 項の組み立て
// =============================================

// Atom は Go の真偽値を Prolog のアトム true/false にする。
func Atom(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Ints は整数列を Prolog のリスト項にする。
func Ints(xs []int) string {
	terms := make([]string, len(xs))
	for i, x := range xs {
		terms[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(terms, ",") + "]"
}

// Atoms はアトム列を Prolog のリスト項にする。
// 要素は固定語彙 (スート名など) であることを前提にしており、引用符は付けない。
func Atoms(xs []string) string {
	return "[" + strings.Join(xs, ",") + "]"
}
