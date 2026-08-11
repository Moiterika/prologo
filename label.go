package main

// label.go — カードの表示用ラベル。
// ブラウザ上なので ♠♥♦♣ をそのまま使える (端末の桁幅問題が無い)。

var suitSymbol = map[string]string{
	"s": "♠",
	"h": "♥",
	"d": "♦",
	"c": "♣",
}

var suitJa = map[string]string{
	"s": "スペード",
	"h": "ハート",
	"d": "ダイヤ",
	"c": "クラブ",
}

var rankLabel = map[string]string{
	"j": "J",
	"q": "Q",
	"k": "K",
	"a": "A",
}

var passDirJa = map[string]string{
	"left":   "左隣",
	"right":  "右隣",
	"across": "向かい",
	"hold":   "交換なし",
}

// CardLabel は "♠A" のような表示文字列。
func CardLabel(c Card) string {
	return suitSymbol[c.Suit] + RankLabel(c.Rank)
}

// RankLabel は "10" や "A" のようなランク表示。
func RankLabel(rank string) string {
	if l, ok := rankLabel[rank]; ok {
		return l
	}
	return rank
}

// SuitSymbol は ♠♥♦♣。
func SuitSymbol(suit string) string { return suitSymbol[suit] }

// SuitName はスートの日本語名。
func SuitName(suit string) string { return suitJa[suit] }

// IsRedSuit は赤いスート (♥♦) か。
func IsRedSuit(suit string) bool { return suit == "h" || suit == "d" }
