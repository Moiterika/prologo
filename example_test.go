package main

// example_test.go — 使い方の実例。
//
// Go の Example 関数は `go test` で実行され、標準出力が末尾の
// 「Output:」コメントと一致するかまで検証される。
// つまりここに書いた出力は、ずれたらテストが落ちる。

import (
	"fmt"
	"log"
	"strings"
)

// labels は ["♣9", "♣5"] のような表示にする。
func labels(cards []Card) string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = CardLabel(c)
	}
	if len(out) == 0 {
		return "(なし)"
	}
	return strings.Join(out, " ")
}

func mustRules() *Rules {
	rules, err := NewRules("")
	if err != nil {
		log.Fatal(err)
	}
	return rules
}

// マストフォローの判定。
// 手札と台札を渡すと、いま出せるカードだけが返る。
//
// 判定しているのは hearts.pl の can_play/3 で、
// 「台札と同じスートを持っていたらそれを出さなければならない」という規則。
func ExampleRules_LegalCards() {
	rules := mustRules()

	// 第 1 トリックには別の制約があるので、ここでは 5 巡目を例にする。
	// (第 4 引数はハートブレイク済みかどうか。フォロー時には影響しない)
	const trickNo = 5
	lead := Card{Suit: "c", Rank: "2"} // 台札は ♣2

	// その1: 台札と同じスート (クラブ) を持っている場合
	hand := []Card{
		{Suit: "s", Rank: "q"},
		{Suit: "h", Rank: "3"},
		{Suit: "c", Rank: "9"},
		{Suit: "c", Rank: "5"},
	}
	legal, err := rules.LegalCards(hand, &lead, trickNo, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("台札:", CardLabel(lead))
	fmt.Println("手札:", labels(hand))
	fmt.Println("出せる:", labels(legal))

	// その2: 台札のスートを 1 枚も持っていない場合は何を出してもよい
	void := []Card{
		{Suit: "s", Rank: "q"},
		{Suit: "h", Rank: "3"},
		{Suit: "d", Rank: "7"},
	}
	legal, err = rules.LegalCards(void, &lead, trickNo, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()
	fmt.Println("台札:", CardLabel(lead))
	fmt.Println("手札:", labels(void))
	fmt.Println("出せる:", labels(legal))

	// Output:
	// 台札: ♣2
	// 手札: ♠Q ♥3 ♣9 ♣5
	// 出せる: ♣9 ♣5
	//
	// 台札: ♣2
	// 手札: ♠Q ♥3 ♦7
	// 出せる: ♠Q ♥3 ♦7
}

// トリックの勝敗判定。
// 出揃ったカードと台札のスートを渡すと、勝ったカードが返る。
//
// 判定しているのは hearts.pl の determine_winner/3 で、
// 「台札と同じスートの中で一番強いカードが勝つ」という規則。
// 台札と違うスートは、どれだけ強くても勝敗に関わらない。
func ExampleRules_DetermineWinner() {
	rules := mustRules()

	// ♣ でリードされたトリック。♥A や ♠Q は台札スートではないので無関係。
	trick := []Card{
		{Suit: "c", Rank: "5"},
		{Suit: "c", Rank: "k"},
		{Suit: "h", Rank: "a"},
		{Suit: "s", Rank: "q"},
	}
	winner, err := rules.DetermineWinner(trick, "c")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("場:", labels(trick))
	fmt.Println("勝ち札:", CardLabel(winner))

	// 誰が取ったかは、勝ち札を出した人を探せば分かる。
	// (このゲームでは 0 番から順に出したものとする)
	for i, c := range trick {
		if c == winner {
			fmt.Printf("取ったのは %d 番目に出した人\n", i+1)
		}
	}

	// 失点は hearts.pl の calculate_score/2 で数える。
	points, err := rules.CalculateScore(trick)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("このトリックの失点:", points)

	// Output:
	// 場: ♣5 ♣K ♥A ♠Q
	// 勝ち札: ♣K
	// 取ったのは 2 番目に出した人
	// このトリックの失点: 14
}
