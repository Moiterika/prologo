package main

// ui.go — CUI の描画と入力。

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// スート表示。表示順もこの並び (♠ ♥ ♦ ♣)。
var suitOrder = []string{"s", "h", "d", "c"}

var suitSymbol = map[string]string{
	"s": "♠",
	"h": "♥",
	"d": "♦",
	"c": "♣",
}

var suitLetter = map[string]string{
	"s": "S",
	"h": "H",
	"d": "D",
	"c": "C",
}

var suitName = map[string]string{
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

var passDirName = map[string]string{
	"left":   "左隣へ",
	"right":  "右隣へ",
	"across": "向かいへ",
	"hold":   "交換なし",
}

// ANSI エスケープ
const (
	ansiReset = "\x1b[0m"
	ansiDim   = "\x1b[90m"
	ansiRed   = "\x1b[31;1m"
	ansiWhite = "\x1b[97m"
	ansiGreen = "\x1b[32;1m"
	ansiCyan  = "\x1b[36;1m"
	ansiBold  = "\x1b[1m"
)

// スート表示モード
const (
	suitModeAuto   = "auto"
	suitModeLetter = "letter"
	suitModeSymbol = "symbol"
	suitModeWide   = "wide"
)

// UI は端末との入出力をまとめる。
type UI struct {
	in    *bufio.Reader
	out   io.Writer
	rules *Rules
	color bool
	auto  bool // デモ再生: 入力も一時停止も行わない

	// letters が真なら ♠♥♦♣ ではなく S/H/D/C を使う。
	// ♠♥♦♣ は East Asian Ambiguous 幅の文字で、端末によって 1 桁にも
	// 2 桁にも描画される。桁揃えが崩れる環境では letters を使う。
	letters bool
	// ambiguousWide は記号表示のとき ♠♥♦♣ を 2 桁として数えるかどうか。
	ambiguousWide bool
}

func (u *UI) printf(format string, a ...interface{}) {
	fmt.Fprintf(u.out, format, a...)
}

func (u *UI) println(a ...interface{}) {
	fmt.Fprintln(u.out, a...)
}

func (u *UI) paint(color, s string) string {
	if !u.color {
		return s
	}
	return color + s + ansiReset
}

// arrow は場のカードを繋ぐ矢印。記号を使わないモードでは ASCII にする。
func (u *UI) arrow() string {
	if u.letters {
		return "  ->  "
	}
	return "  →  "
}

// suitMark は行頭などに出すスートの表示。
func (u *UI) suitMark(suit string) string {
	if u.letters {
		return suitLetter[suit]
	}
	return suitSymbol[suit]
}

// =============================================
// 表示幅
// =============================================

// width は端末上の表示桁数を数える。
func (u *UI) width(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case isFullwidth(r):
			w += 2
		case u.ambiguousWide && isAmbiguousWidth(r):
			w += 2
		default:
			w++
		}
	}
	return w
}

// pad は表示幅を揃えて右側に空白を足す。
// 日本語を %-8s で揃えるとバイト数で数えられてずれるため。
func (u *UI) pad(s string, width int) string {
	if w := u.width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// isFullwidth は「どの端末でも 2 桁」の文字か。
func isFullwidth(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // ハングル字母
		r >= 0x2E80 && r <= 0xA4CF, // CJK 部首・かな・漢字
		r >= 0xAC00 && r <= 0xD7A3, // ハングル音節
		r >= 0xF900 && r <= 0xFAFF, // CJK 互換漢字
		r >= 0xFE30 && r <= 0xFE6F, // CJK 互換形
		r >= 0xFF00 && r <= 0xFF60, // 全角英数記号
		r >= 0xFFE0 && r <= 0xFFE6:
		return true
	}
	return false
}

// isAmbiguousWidth は端末次第で 1 桁にも 2 桁にもなる文字か。
// ♠♥♦♣ や → がここに入る。
func isAmbiguousWidth(r rune) bool {
	return r >= 0x2000 && r <= 0x2BFF
}

// =============================================
// カードの表示
// =============================================

// cardText は "♠A" のような 1 枚の表示文字列を返す (色なし)。
func (u *UI) cardText(c Card) string {
	rank := c.Rank
	if l, ok := rankLabel[rank]; ok {
		rank = l
	}
	return u.suitMark(c.Suit) + rank
}

// cardCell は表示幅 4 桁に揃えた 1 枚分のセル。
func (u *UI) cardCell(c Card) string {
	text := u.cardText(c)
	body := u.pad(text, 4)
	if c.Suit == "h" || c.Suit == "d" {
		return u.paint(ansiRed, body)
	}
	return u.paint(ansiWhite, body)
}

// sortForDisplay は ♠♥♦♣ の順、各スート内はランクの強い順に並べる。
// ランクの強さは hearts.pl の rank_value/2 に由来する。
func (u *UI) sortForDisplay(cards []Card) []Card {
	out := append([]Card(nil), cards...)
	suitRank := map[string]int{}
	for i, s := range suitOrder {
		suitRank[s] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		if suitRank[out[i].Suit] != suitRank[out[j].Suit] {
			return suitRank[out[i].Suit] < suitRank[out[j].Suit]
		}
		return u.rules.RankValue(out[i].Rank) > u.rules.RankValue(out[j].Rank)
	})
	return out
}

// showHand は手札を番号付きで表示する。legal が nil でなければ、
// 出せるカードだけを強調して表示する。
// 返り値は「表示番号 -> カード」の対応 (1 始まり)。
func (u *UI) showHand(hand []Card, legal []Card) []Card {
	shown := u.sortForDisplay(hand)

	legalSet := map[Card]bool{}
	for _, c := range legal {
		legalSet[c] = true
	}
	marking := legal != nil

	u.printf("%s\n", u.paint(ansiBold, "あなたの手札"))

	// スートごとに 1 行
	idx := 0
	var legalNums []string
	for _, suit := range suitOrder {
		var cells []string
		for i, c := range shown {
			if c.Suit != suit {
				continue
			}
			num := fmt.Sprintf("%2d:", i+1)
			cell := u.cardCell(c)
			if marking {
				if legalSet[c] {
					num = u.paint(ansiGreen, num)
					legalNums = append(legalNums, strconv.Itoa(i+1))
				} else {
					num = u.paint(ansiDim, num)
					cell = u.paint(ansiDim, u.pad(u.cardText(c), 4))
				}
			}
			cells = append(cells, num+cell)
			idx++
		}
		if len(cells) == 0 {
			continue
		}
		u.printf("  %s %s\n", u.suitMark(suit), strings.Join(cells, " "))
	}
	if idx == 0 {
		u.println("  (なし)")
	}

	// 色が使えない環境でも分かるよう、出せる番号を必ず並べて示す。
	if marking {
		u.printf("  %s %s\n",
			u.paint(ansiGreen, "出せる番号:"), strings.Join(legalNums, " "))
	}
	return shown
}

// showTrick は場に出ているカードを表示する。
func (u *UI) showTrick(names []string, players []int, cards []Card, trickNo, total int) {
	u.printf("%s\n", u.paint(ansiBold, fmt.Sprintf("--- 第 %d / %d トリック ---", trickNo, total)))
	if len(cards) == 0 {
		u.println("  場: (まだ誰も出していません)")
		return
	}
	var parts []string
	for i, c := range cards {
		parts = append(parts, fmt.Sprintf("%s %s", names[players[i]], strings.TrimSpace(u.cardCell(c))))
	}
	u.printf("  場: %s\n", strings.Join(parts, u.arrow()))
}

// showScoreTable は現在の累計得点を表示する。
func (u *UI) showScoreTable(names []string, scores []int, limit int) {
	u.printf("\n%s (%d点で終了)\n", u.paint(ansiBold, "累計失点"), limit)
	for i, n := range names {
		bar := strings.Repeat("#", scores[i]/2)
		u.printf("  %s %4d  %s\n", u.pad(n, 8), scores[i], u.paint(ansiCyan, bar))
	}
	u.println()
}

// =============================================
// 入力
// =============================================

// askInt は範囲内の整数を 1 つ読む。
func (u *UI) askInt(prompt string, min, max int) (int, error) {
	for {
		u.printf("%s ", prompt)
		line, err := u.readLine()
		if err != nil {
			return 0, err
		}
		line = strings.TrimSpace(line)
		n, convErr := strconv.Atoi(line)
		if convErr != nil || n < min || n > max {
			u.printf("  %d〜%d の番号を入力してください。\n", min, max)
			continue
		}
		return n, nil
	}
}

// askInts は count 個の相異なる整数を読む。
func (u *UI) askInts(prompt string, count, min, max int) ([]int, error) {
	for {
		u.printf("%s ", prompt)
		line, err := u.readLine()
		if err != nil {
			return nil, err
		}
		fields := strings.FieldsFunc(strings.TrimSpace(line), func(r rune) bool {
			return r == ' ' || r == ',' || r == '\t' || r == '、' || r == '　'
		})
		if len(fields) != count {
			u.printf("  番号を %d 個、スペース区切りで入力してください。\n", count)
			continue
		}
		seen := map[int]bool{}
		nums := make([]int, 0, count)
		ok := true
		for _, f := range fields {
			n, convErr := strconv.Atoi(f)
			if convErr != nil || n < min || n > max || seen[n] {
				ok = false
				break
			}
			seen[n] = true
			nums = append(nums, n)
		}
		if !ok {
			u.printf("  %d〜%d の相異なる番号を %d 個入力してください。\n", min, max, count)
			continue
		}
		return nums, nil
	}
}

func (u *UI) readLine() (string, error) {
	line, err := u.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return line, nil
}

// pause は Enter 待ち。デモ再生中は何もしない。
func (u *UI) pause() {
	if u.auto {
		return
	}
	u.printf("%s", u.paint(ansiDim, "  [Enter で次へ] "))
	_, _ = u.readLine()
}
