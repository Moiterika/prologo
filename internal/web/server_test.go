package web

// server_test.go — HTTP 層の検証。
// 実際に httptest でサーバーを立て、フォーム送信でゲームを進める。

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	mux := http.NewServeMux()
	NewServer("", 0, 100, 0).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, &http.Client{Jar: jar}
}

func get(t *testing.T, c *http.Client, url string) string {
	t.Helper()
	res, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", url, res.StatusCode, body)
	}
	return string(body)
}

func post(t *testing.T, c *http.Client, url string, form url.Values) string {
	t.Helper()
	res, err := c.PostForm(url, form)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST %s = %d\n%s", url, res.StatusCode, body)
	}
	return string(body)
}

// playableNumbers は画面から「押せるカードの番号」を拾う。
var playableRe = regexp.MustCompile(`name="card" value="(\d+)"(?:\s+class=)`)

func playableNumbers(html string) []string {
	var out []string
	for _, m := range playableRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// startGame は最初のルール選択を済ませてゲームを開始する。
func startGame(t *testing.T, c *http.Client, base string, points, lead, heartsBreak string) string {
	t.Helper()
	return post(t, c, base+"/start", url.Values{
		"points":       {points},
		"lead":         {lead},
		"hearts_break": {heartsBreak},
	})
}

// startDefaultGame は既定の流儀でゲームを開始する。
func startDefaultGame(t *testing.T, c *http.Client, base string) string {
	t.Helper()
	return startGame(t, c, base, "allow", "opening_card", "off")
}

// 最初の画面はルール選択。
func TestServerShowsSetupFirst(t *testing.T) {
	srv, c := newTestServer(t)

	body := get(t, c, srv.URL+"/")
	for _, want := range []string{
		"ルールを選んでください",
		`value="forbid_all"`,
		`value="forbid_hearts"`,
		`value="allow"`,
		`value="opening_card"`,
		`value="free"`,
		"ハートブレイク（得点札が出るまで♥をリードできない）",
		// 既定の選択
		`name="points" value="allow" checked`,
		`name="lead" value="opening_card" checked`,
		`name="hearts_break" value="off" checked`,
		`name="hearts_break" value="on"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ルール選択画面に %q がありません", want)
		}
	}

	// 表示順: リード → 点数札 → ハートブレイク
	inOrder(t, body,
		"第1トリックのリード",
		"第1トリックに点数札を出せるか",
		"ハートブレイク（")

	// 点数札は制限のゆるい順
	inOrder(t, body,
		`value="allow"`,
		`value="forbid_hearts"`,
		`value="forbid_all"`)

	if len(c.Jar.Cookies(mustParse(t, srv.URL))) == 0 {
		t.Error("セッション Cookie が発行されていません")
	}
}

// inOrder は与えた文字列がその順に現れることを確かめる。
func inOrder(t *testing.T, body string, parts ...string) {
	t.Helper()
	prev := -1
	for _, p := range parts {
		i := strings.Index(body, p)
		if i < 0 {
			t.Errorf("%q が画面にありません", p)
			return
		}
		if i < prev {
			t.Errorf("%q の位置が想定より前です (順序: %v)", p, parts)
			return
		}
		prev = i
	}
}

func TestServerRendersTable(t *testing.T) {
	srv, c := newTestServer(t)

	get(t, c, srv.URL+"/")
	body := startDefaultGame(t, c, srv.URL)

	// タイトルは部分一致だと「ハートブレイク」等にも当たるので要素ごと見る
	for _, want := range []string{"<title>ハート</title>", "あなた", "CPU1", "第 1 ラウンド"} {
		if !strings.Contains(body, want) {
			t.Errorf("盤面に %q がありません", want)
		}
	}
}

func TestServerPlaysThroughATrick(t *testing.T) {
	srv, c := newTestServer(t)

	get(t, c, srv.URL+"/")
	body := startDefaultGame(t, c, srv.URL)

	// 交換の局面なら 3 枚渡す
	if strings.Contains(body, "渡すカードを") {
		body = post(t, c, srv.URL+"/pass", url.Values{"card": {"1", "2", "3"}})
	}

	// 人間の手番が来るまで進め、押せるカードを 1 枚出す
	if !strings.Contains(body, "出すカードを選んでください") {
		t.Fatalf("手番が回ってきません:\n%s", firstLines(body, 40))
	}
	nums := playableNumbers(body)
	if len(nums) == 0 {
		t.Fatal("押せるカードが 1 枚もありません")
	}
	body = post(t, c, srv.URL+"/play", url.Values{"card": {nums[0]}})

	if !strings.Contains(body, "が取りました") {
		t.Fatalf("トリックが解決していません:\n%s", firstLines(body, 40))
	}

	// 次のトリックへ
	body = post(t, c, srv.URL+"/next", nil)
	if !strings.Contains(body, "第 2 / 13 トリック") && !strings.Contains(body, "ラウンド結果") {
		t.Fatalf("次のトリックに進んでいません:\n%s", firstLines(body, 40))
	}
}

func TestServerShowsInputError(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/")
	startDefaultGame(t, c, srv.URL)

	// 交換の局面で 1 枚しか選ばずに送る
	body := post(t, c, srv.URL+"/pass", url.Values{"card": {"1"}})
	if !strings.Contains(body, "枚選んでください") {
		t.Fatalf("入力ミスの指摘が出ていません:\n%s", firstLines(body, 40))
	}

	// 500 ではなく画面が返ること自体が期待動作 (post が確認済み)
}

// 選んだ流儀が盤面に反映される。
func TestServerAppliesChosenRules(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/")

	body := startGame(t, c, srv.URL, "allow", "free", "on")
	for _, want := range []string{
		"第1トリックの点数札制限なし",
		"♣2 の持ち主が自由にリード",
		"ハートブレイクあり",
		"♥ 未ブレイク", // 規則を使う設定なので状態表示が出る
	} {
		if !strings.Contains(body, want) {
			t.Errorf("盤面に %q がありません", want)
		}
	}

	// 選び直すとルール選択に戻り、前回の選択が残っている
	body = post(t, c, srv.URL+"/setup", nil)
	if !strings.Contains(body, "ルールを選んでください") {
		t.Fatalf("ルール選択に戻っていません:\n%s", firstLines(body, 40))
	}
	for _, want := range []string{
		`value="allow" checked`,
		`value="free" checked`,
		`name="hearts_break" value="on" checked`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("前回の選択 %q が残っていません", want)
		}
	}
}

// ハートブレイク規則を使わない設定 (既定) では、状態表示も知らせも出さない。
func TestServerHidesHeartsBreakWhenOff(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/")

	body := startDefaultGame(t, c, srv.URL)
	if !strings.Contains(body, "ハートブレイクなし") {
		t.Error("盤面に「ハートブレイクなし」がありません")
	}
	for _, unwanted := range []string{"♥ 未ブレイク", "♥ ブレイク済み"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("規則を使わない設定なのに %q が出ています", unwanted)
		}
	}
}

func TestSessionsAreIndependent(t *testing.T) {
	srv, c1 := newTestServer(t)

	jar, _ := cookiejar.New(nil)
	c2 := &http.Client{Jar: jar}

	get(t, c1, srv.URL+"/")
	get(t, c2, srv.URL+"/")

	// c1 だけゲームを開始する
	startDefaultGame(t, c1, srv.URL)

	body2 := get(t, c2, srv.URL+"/")
	if !strings.Contains(body2, "ルールを選んでください") {
		t.Fatal("別セッションのゲームが巻き込まれています")
	}
}

func TestNewGameResets(t *testing.T) {
	srv, c := newTestServer(t)
	get(t, c, srv.URL+"/")
	startDefaultGame(t, c, srv.URL)
	post(t, c, srv.URL+"/pass", url.Values{"card": {"1", "2", "3"}})

	body := post(t, c, srv.URL+"/new", nil)
	if !strings.Contains(body, "第 1 ラウンド") {
		t.Fatalf("やり直しで第1ラウンドに戻っていません:\n%s", firstLines(body, 40))
	}
	if !strings.Contains(body, "渡すカードを") {
		t.Fatalf("やり直しで交換の局面に戻っていません:\n%s", firstLines(body, 40))
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
