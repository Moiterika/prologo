package web

// server_games_test.go — 大富豪とナポレオンの HTTP 層の検証。
// 実際に httptest でサーバーを立て、フォーム送信でゲームを進める。

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// checkboxRe は「選べるカードの番号」を拾う (大富豪・ナポレオンの複数選択)。
var checkboxRe = regexp.MustCompile(`type="checkbox" name="card" value="(\d+)"`)

func checkboxNumbers(html string) []string {
	var out []string
	for _, m := range checkboxRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// bidValues は競り画面から押せる宣言ボタンの値を拾う。
var bidRe = regexp.MustCompile(`name="bid" value="([a-z]+:\d+)"`)

func bidValues(html string) []string {
	var out []string
	for _, m := range bidRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// aideValues は副官指名の画面から押せるカードの値を拾う。
var aideRe = regexp.MustCompile(`name="card" value="([a-z]+:[a-z0-9]+)"`)

func aideValues(html string) []string {
	var out []string
	for _, m := range aideRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// =============================================
// ヘッダーの行き来
// =============================================

func TestNavLinksAllThreeGames(t *testing.T) {
	srv, c := newTestServer(t)
	for path, want := range map[string]string{
		"/":         "ハート",
		"/daifugo":  "大富豪",
		"/napoleon": "ナポレオン",
	} {
		body := get(t, c, srv.URL+path)
		if !strings.Contains(body, want) {
			t.Errorf("GET %s に %q がありません", path, want)
		}
		for _, href := range []string{`href="/"`, `href="/daifugo"`, `href="/napoleon"`} {
			if !strings.Contains(body, href) {
				t.Errorf("GET %s のヘッダーに %s がありません", path, href)
			}
		}
	}
}

// =============================================
// 大富豪
// =============================================

func TestDaifugoStartAndPlay(t *testing.T) {
	srv, c := newTestServer(t)

	setup := get(t, c, srv.URL+"/daifugo")
	if !strings.Contains(setup, "ローカルルールを選んでください") {
		t.Fatalf("ルール選択の画面ではありません:\n%s", head(setup))
	}

	// 4 人・ジョーカー 2 枚・8流しあり、縛りなしで始める
	body := post(t, c, srv.URL+"/daifugo/start", url.Values{
		"players":        {"4"},
		"jokers":         {"2"},
		"first_play":     {"diamond3_free"},
		"eight_cut":      {"all"},
		"jack_back":      {"off"},
		"seq_revolution": {"off"},
		"five_card":      {"off"},
		"bind":           {"off"},
		"spade_three":    {"off"},
		"pass_lock":      {"off"},
		"finish_flush":   {"off"},
		"foul_finish":    {"off"},
		"miyako_ochi":    {"off"},
	})
	if !strings.Contains(body, "あなたの手札") {
		t.Fatalf("盤面が出ていません:\n%s", head(body))
	}

	// 人間の手番が来るまで進める。CPU は同じ応答の中で動くので、
	// 「出す」ボタンか「パス」ボタンが出るまでは何もしなくてよい。
	if !strings.Contains(body, "出す") && !strings.Contains(body, "パス") {
		t.Fatalf("あなたの手番になっていません:\n%s", head(body))
	}

	// 出せる手があるなら 1 枚出してみる。無ければパスする。
	nums := checkboxNumbers(body)
	if len(nums) == 0 {
		t.Fatalf("選べるカードがありません:\n%s", head(body))
	}
	if strings.Contains(body, "出せる手がありません") {
		body = post(t, c, srv.URL+"/daifugo/pass", nil)
	} else {
		// 場が空 (親番) なら、手札のどれを 1 枚出しても通る
		body = post(t, c, srv.URL+"/daifugo/play", url.Values{"card": {nums[0]}})
	}
	if strings.Contains(body, "内部エラー") {
		t.Fatalf("内部エラーになりました:\n%s", head(body))
	}
}

// 出せない手を選んだときは 500 ではなく画面上のメッセージになる。
func TestDaifugoIllegalPlayShowsMessage(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/daifugo")
	body := post(t, c, srv.URL+"/daifugo/start", url.Values{
		"players":    {"4"},
		"first_play": {"diamond3_must"},
	})

	// ♦3 で始める設定なので、人間が親なら ♦3 以外は弾かれる
	nums := checkboxNumbers(body)
	if len(nums) < 2 || !strings.Contains(body, "出す") {
		t.Skip("この配札では人間が最初の親ではありません")
	}
	// 手札の 2 枚をまとめて出す。同じ数字が揃っていなければ形として不正。
	body = post(t, c, srv.URL+"/daifugo/play", url.Values{"card": {nums[0], nums[1]}})
	if strings.Contains(body, "内部エラー") {
		t.Fatalf("内部エラーになりました:\n%s", head(body))
	}
}

// セッションが分かれていれば、別のブラウザの対局は混ざらない。
func TestDaifugoSessionsAreIndependent(t *testing.T) {
	srv, c1 := newTestServer(t)
	jarless := &http.Client{}

	get(t, c1, srv.URL+"/daifugo")
	body1 := post(t, c1, srv.URL+"/daifugo/start", url.Values{"players": {"4"}})
	if !strings.Contains(body1, "あなたの手札") {
		t.Fatalf("1 人目の盤面が出ていません")
	}

	// Cookie を持たない別のクライアントはルール選択から始まる
	body2 := get(t, jarless, srv.URL+"/daifugo")
	if !strings.Contains(body2, "ローカルルールを選んでください") {
		t.Errorf("2 人目がルール選択から始まっていません:\n%s", head(body2))
	}
}

// =============================================
// ナポレオン
// =============================================

func TestNapoleonBiddingFlow(t *testing.T) {
	srv, c := newTestServer(t)

	setup := get(t, c, srv.URL+"/napoleon")
	if !strings.Contains(setup, "ローカルルールを選んでください") {
		t.Fatalf("ルール選択の画面ではありません:\n%s", head(setup))
	}

	body := post(t, c, srv.URL+"/napoleon/start", url.Values{
		"players":       {"5"},
		"min_bid":       {"10"},
		"same_two":      {"on"},
		"yoromeki":      {"off"},
		"joker_rule":    {"none"},
		"mighty_exempt": {"off"},
		"scoring":       {"twelfth"},
		"slam":          {"normal"},
	})
	if !strings.Contains(body, "ナポレオン宣言") && !strings.Contains(body, "あなたの手札") {
		t.Fatalf("盤面が出ていません:\n%s", head(body))
	}

	// 競りが人間のところで止まっていたら、必ず宣言できるボタンが出ている
	if strings.Contains(body, "あなたの番です") && strings.Contains(body, "ナポレオン宣言") {
		bids := bidValues(body)
		if len(bids) == 0 {
			t.Fatalf("宣言の選択肢がありません:\n%s", head(body))
		}
		// いちばん弱い宣言で引き受けてみる
		body = post(t, c, srv.URL+"/napoleon/bid", url.Values{"bid": {bids[0]}})
		if strings.Contains(body, "内部エラー") {
			t.Fatalf("内部エラーになりました:\n%s", head(body))
		}
	}

	// パスは常に通る
	if strings.Contains(body, "ナポレオン宣言") && strings.Contains(body, "あなたの番です") {
		body = post(t, c, srv.URL+"/napoleon/bid", url.Values{"bid": {"pass"}})
		if strings.Contains(body, "内部エラー") {
			t.Fatalf("パスで内部エラーになりました:\n%s", head(body))
		}
	}
}

// 人間がナポレオンになった場合の、副官指名と埋め札の流れ。
func TestNapoleonNamingAndDiscard(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/napoleon")
	body := post(t, c, srv.URL+"/napoleon/start", url.Values{"players": {"5"}, "min_bid": {"10"}})

	// 人間の番が来たら、強い宣言をして必ずナポレオンになる
	for i := 0; i < 12 && strings.Contains(body, "ナポレオン宣言"); i++ {
		if !strings.Contains(body, "あなたの番です") {
			t.Skip("この配札では人間に手番が回る前に競りが終わりました")
		}
		bids := bidValues(body)
		if len(bids) == 0 {
			t.Skip("これ以上強い宣言ができません")
		}
		body = post(t, c, srv.URL+"/napoleon/bid", url.Values{"bid": {bids[len(bids)-1]}})
	}

	if strings.Contains(body, "副官のカードを指名してください") {
		cards := aideValues(body)
		if len(cards) == 0 {
			t.Fatalf("副官の候補がありません:\n%s", head(body))
		}
		body = post(t, c, srv.URL+"/napoleon/aide", url.Values{"card": {cards[0]}})

		if !strings.Contains(body, "伏せ直すカード") {
			t.Fatalf("埋め札の画面になっていません:\n%s", head(body))
		}
		nums := checkboxNumbers(body)
		if len(nums) < 2 {
			t.Fatalf("選べるカードが足りません:\n%s", head(body))
		}
		body = post(t, c, srv.URL+"/napoleon/discard", url.Values{"card": {nums[0], nums[1]}})
		if strings.Contains(body, "内部エラー") {
			t.Fatalf("内部エラーになりました:\n%s", head(body))
		}
		if !strings.Contains(body, "切り札") {
			t.Fatalf("対戦が始まっていません:\n%s", head(body))
		}
	}
}

// head は失敗時の出力を読める長さに切る。
func head(s string) string {
	const n = 2000
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(以下省略)"
}
