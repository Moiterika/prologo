package web

// server_preset_test.go — ルール選択画面のプリセットと、
// プレイ中に終了してゲーム選択へ戻るボタンの検証。

import (
	"net/url"
	"strings"
	"testing"
)

// checkedValue は設定画面で name のラジオボタンのうち選ばれている値を返す。
func checkedValue(t *testing.T, html, name string) string {
	t.Helper()
	for _, part := range strings.Split(html, "<input")[1:] {
		tag := part[:strings.Index(part, ">")]
		if strings.Contains(tag, `name="`+name+`"`) && strings.Contains(tag, "checked") {
			i := strings.Index(tag, `value="`) + len(`value="`)
			return tag[i : i+strings.Index(tag[i:], `"`)]
		}
	}
	t.Fatalf("%s が選ばれていません", name)
	return ""
}

func TestHeartsPresets(t *testing.T) {
	srv, c := newTestServer(t)

	// 既定の流儀は「基本ルールのみ」と同じ。
	setup := get(t, c, srv.URL+"/hearts")
	if !strings.Contains(setup, "作者オススメ") || !strings.Contains(setup, "選択中") {
		t.Fatalf("ルール選択の画面にプリセットがありません:\n%s", head(setup))
	}

	body := post(t, c, srv.URL+"/hearts/preset", url.Values{"preset": {"recommended"}})
	if !strings.Contains(body, "「作者オススメ」の組み合わせにしました") {
		t.Fatalf("プリセットの案内がありません:\n%s", head(body))
	}
	for name, want := range map[string]string{
		"lead":         "opening_card",
		"points":       "forbid_all",
		"hearts_break": "on",
	} {
		if got := checkedValue(t, body, name); got != want {
			t.Errorf("作者オススメで %s = %s, want %s", name, got, want)
		}
	}

	body = post(t, c, srv.URL+"/hearts/preset", url.Values{"preset": {"basic"}})
	for name, want := range map[string]string{
		"lead":         "opening_card",
		"points":       "allow",
		"hearts_break": "off",
	} {
		if got := checkedValue(t, body, name); got != want {
			t.Errorf("基本ルールのみで %s = %s, want %s", name, got, want)
		}
	}
}

func TestDaifugoPresets(t *testing.T) {
	srv, c := newTestServer(t)

	setup := get(t, c, srv.URL+"/daifugo")
	for _, want := range []string{"基本ルールのみ", "作者オススメ", "選択中"} {
		if !strings.Contains(setup, want) {
			t.Fatalf("ルール選択の画面に %q がありません", want)
		}
	}

	// 人数を変えてからプリセットを押しても、人数は残る。
	post(t, c, srv.URL+"/daifugo/start", url.Values{"players": {"5"}})
	post(t, c, srv.URL+"/daifugo/setup", nil)
	body := post(t, c, srv.URL+"/daifugo/preset", url.Values{"preset": {"basic"}})
	if !strings.Contains(body, "「基本ルールのみ」の組み合わせにしました") {
		t.Fatalf("プリセットの案内がありません:\n%s", head(body))
	}
	for name, want := range map[string]string{
		"players":     "5",
		"eight_cut":   "off",
		"jack_back":   "off",
		"bind":        "off",
		"ten_discard": "off",
		"miyako_ochi": "off",
	} {
		if got := checkedValue(t, body, name); got != want {
			t.Errorf("基本ルールのみで %s = %s, want %s", name, got, want)
		}
	}

	body = post(t, c, srv.URL+"/daifugo/preset", url.Values{"preset": {"recommended"}})
	if got := checkedValue(t, body, "eight_cut"); got != "all" {
		t.Errorf("作者オススメで eight_cut = %s, want all", got)
	}
}

func TestNapoleonPresets(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/napoleon")

	body := post(t, c, srv.URL+"/napoleon/preset", url.Values{"preset": {"basic"}})
	for name, want := range map[string]string{
		"same_two":      "off",
		"yoromeki":      "off",
		"joker_rule":    "none",
		"mighty_exempt": "off",
	} {
		if got := checkedValue(t, body, name); got != want {
			t.Errorf("基本ルールのみで %s = %s, want %s", name, got, want)
		}
	}

	// 基本ルールのままで始められる。
	body = post(t, c, srv.URL+"/napoleon/start", url.Values{
		"players": {"5"}, "min_bid": {"10"}, "same_two": {"off"}, "yoromeki": {"off"},
		"joker_rule": {"none"}, "mighty_exempt": {"off"}, "scoring": {"twelfth"}, "slam": {"normal"},
	})
	if !strings.Contains(body, "終了してゲーム選択へ") {
		t.Fatalf("盤面に終了ボタンがありません:\n%s", head(body))
	}
}

func TestQuitReturnsToHome(t *testing.T) {
	srv, c := newTestServer(t)

	for _, g := range []struct{ game, label string }{
		{"/hearts", "ハート"},
		{"/daifugo", "大富豪"},
		{"/napoleon", "ナポレオン"},
	} {
		body := post(t, c, srv.URL+g.game+"/start", nil)
		quit := `action="` + g.game + `/quit"`
		if !strings.Contains(body, quit) || !strings.Contains(body, "confirm(") {
			t.Fatalf("%s の盤面に確認つきの終了ボタンがありません:\n%s", g.game, head(body))
		}
		if home := get(t, c, srv.URL+"/"); !strings.Contains(home, "対局中") {
			t.Errorf("%s の対局中にゲーム選択の画面へ印が出ていません", g.label)
		}

		// 終了するとゲームを選ぶ画面へリダイレクトされ、対局は消える。
		body = post(t, c, srv.URL+g.game+"/quit", nil)
		if !strings.Contains(body, "遊ぶゲームを選んでください") {
			t.Errorf("%s を終了したあとがゲーム選択の画面ではありません:\n%s", g.label, head(body))
		}
		if strings.Contains(body, "対局中") {
			t.Errorf("%s を終了したのに対局中の印が残っています", g.label)
		}
		if setup := get(t, c, srv.URL+g.game); !strings.Contains(setup, "ルールを選んでください") {
			t.Errorf("%s を終了したあと、開き直してもルール選択の画面になりません", g.label)
		}
	}
}
