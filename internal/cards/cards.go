// Package cards はトランプ 1 枚の表現と、Prolog の項との相互変換を提供する。
//
// 3 つのゲーム (ハート・大富豪・ナポレオン) はどれも Prolog 側で
// card(Suit, Rank) という同じ形の項を使う。ジョーカーはスートを joker、
// ランクを 1 始まりの通し番号にした card(joker, 1) / card(joker, 2) で表す。
// ジョーカーを使わないハートではこの形は現れない。
package cards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ichiban/prolog/engine"
)

// JokerSuit はジョーカーを表すスート。Prolog 側の card(joker, N) に対応する。
const JokerSuit = "joker"

// Card は Prolog の card(Suit, Rank) に対応する。
// Suit は "s"/"h"/"d"/"c"、または JokerSuit。
// Rank は "2".."10","j","q","k","a"、ジョーカーでは "1"/"2"。
type Card struct {
	Suit string
	Rank string
}

// IsJoker はジョーカーか。
func (c Card) IsJoker() bool { return c.Suit == JokerSuit }

// Term は Prolog の項テキストを返す。スート・ランクは固定語彙なので
// そのまま埋め込んでよい。
func (c Card) Term() string {
	return "card(" + c.Suit + "," + c.Rank + ")"
}

var atomCard = engine.NewAtom("card")

// Scan は prolog.Scanner を実装し、card(S, R) を Card に変換する。
// ランクは 2..10 が整数、j/q/k/a がアトムなので両方を受ける。
func (c *Card) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	t := env.Resolve(term)
	cmp, ok := t.(engine.Compound)
	if !ok || cmp.Functor() != atomCard || cmp.Arity() != 2 {
		return fmt.Errorf("card/2 ではない項です: %v", t)
	}
	suit, ok := AtomOrIntText(cmp.Arg(0), env)
	if !ok {
		return fmt.Errorf("スートを解釈できません: %v", cmp.Arg(0))
	}
	rank, ok := AtomOrIntText(cmp.Arg(1), env)
	if !ok {
		return fmt.Errorf("ランクを解釈できません: %v", cmp.Arg(1))
	}
	c.Suit, c.Rank = suit, rank
	return nil
}

// AtomOrIntText はアトムと整数のどちらでも来る項を文字列にする。
func AtomOrIntText(term engine.Term, env *engine.Env) (string, bool) {
	switch v := env.Resolve(term).(type) {
	case engine.Atom:
		return v.String(), true
	case engine.Integer:
		return strconv.Itoa(int(v)), true
	default:
		return "", false
	}
}

// List は Go のカード列を Prolog のリスト項に変換する。
func List(cs []Card) string {
	terms := make([]string, len(cs))
	for i, c := range cs {
		terms[i] = c.Term()
	}
	return "[" + strings.Join(terms, ",") + "]"
}

// ListOfLists はカード束の列を Prolog のリストのリストに変換する。
func ListOfLists(groups [][]Card) string {
	terms := make([]string, len(groups))
	for i, g := range groups {
		terms[i] = List(g)
	}
	return "[" + strings.Join(terms, ",") + "]"
}

// Remove は手札から 1 枚を取り除いた新しいスライスを返す。
// 元のスライスは変更しない。同じカードが無ければそのまま返す。
func Remove(hand []Card, card Card) []Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}

// RemoveAll は複数枚をまとめて取り除く。
func RemoveAll(hand []Card, picked []Card) []Card {
	out := append([]Card(nil), hand...)
	for _, c := range picked {
		out = Remove(out, c)
	}
	return out
}

// IndexOf は cs の中で target が何番目かを返す。無ければ -1。
func IndexOf(cs []Card, target Card) int {
	for i, c := range cs {
		if c == target {
			return i
		}
	}
	return -1
}

// Contains は target が含まれるか。
func Contains(cs []Card, target Card) bool { return IndexOf(cs, target) >= 0 }

// =============================================
// 表示用ラベル
// =============================================
// ブラウザ上なので ♠♥♦♣ をそのまま使える (端末の桁幅問題が無い)。

var suitSymbol = map[string]string{
	"s": "♠",
	"h": "♥",
	"d": "♦",
	"c": "♣",
}

var suitJa = map[string]string{
	"s":       "スペード",
	"h":       "ハート",
	"d":       "ダイヤ",
	"c":       "クラブ",
	JokerSuit: "ジョーカー",
}

var rankLabel = map[string]string{
	"j": "J",
	"q": "Q",
	"k": "K",
	"a": "A",
}

// Label は "♠A" のような表示文字列。ジョーカーは "JOKER"。
func Label(c Card) string {
	if c.IsJoker() {
		return "JOKER"
	}
	return suitSymbol[c.Suit] + RankLabel(c.Rank)
}

// RankLabel は "10" や "A" のようなランク表示。
func RankLabel(rank string) string {
	if l, ok := rankLabel[rank]; ok {
		return l
	}
	return rank
}

// SuitSymbol は ♠♥♦♣。ジョーカーは "JOKER"。
func SuitSymbol(suit string) string {
	if suit == JokerSuit {
		return "JOKER"
	}
	return suitSymbol[suit]
}

// SuitName はスートの日本語名。
func SuitName(suit string) string { return suitJa[suit] }

// IsRedSuit は赤いスート (♥♦) か。
func IsRedSuit(suit string) bool { return suit == "h" || suit == "d" }

// Text はアトムと整数のどちらでも来る項を文字列として読む
// prolog.Scanner。ランクのように 10 が整数、j がアトムになる値に使う。
//
// prolog の既定の変換は *string に対して fmt.Stringer しか受け付けず、
// 整数を読もうとすると失敗する。
type Text string

// Scan は prolog.Scanner を実装する。
func (t *Text) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	s, ok := AtomOrIntText(term, env)
	if !ok {
		return fmt.Errorf("アトムでも整数でもない項です: %v", env.Resolve(term))
	}
	*t = Text(s)
	return nil
}

// String はそのままの文字列。
func (t Text) String() string { return string(t) }
