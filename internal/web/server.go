package web

// server.go — HTTP ハンドラ。
//
// WebSocket もデータベースも使わない。ブラウザからの POST でゲームを 1 手
// 進め、その場で画面を描き直して返すだけ。htmx が #app だけを差し替える。
// 状態はプロセス内のメモリに、Cookie のセッション ID で紐付けて持つ。
//
// 3 つのゲーム (ハート / 大富豪 / ナポレオン) を 1 つのサーバーで出す。
// "/" はゲームを選ぶ画面で、各ゲームは "/hearts" "/daifugo" "/napoleon" の下。
// ハートのハンドラはこのファイル、他は server_daifugo.go / server_napoleon.go にある。

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	dgame "hearts/internal/game/daifugo"
	ngame "hearts/internal/game/napoleon"

	"hearts/internal/game"
	"hearts/internal/rules"
	drules "hearts/internal/rules/daifugo"
	nrules "hearts/internal/rules/napoleon"
	"hearts/internal/web/view"
)

const sessionCookie = "cards_session"

// session は 1 ブラウザ分の対局。
//
// Prolog インタプリタはゲームごと・セッションごとに 1 つ持つ。共有すると
// 同時アクセスで状態が混ざるおそれがあるうえ、人数やローカルルールの設定も
// セッション単位で変えられなくなる。
//
// インタプリタはそのゲームを最初に開いたときに作る。ハートしか遊ばない人に
// 大富豪とナポレオンの .pl を読ませても仕方がない。
type session struct {
	mu sync.Mutex

	// shortID はログに出すための短い目印。どのブラウザの対局かを見分ける。
	shortID string

	// ハート
	engine *rules.Engine
	opts   rules.Options
	game   *game.Game // nil ならまだルール選択の画面

	// 大富豪
	dEngine *drules.Engine
	dOpts   drules.Options
	dGame   *dgame.Game

	// ナポレオン
	nEngine *nrules.Engine
	nOpts   nrules.Options
	nGame   *ngame.Game
}

// Config はサーバーの設定。
type Config struct {
	PLDir   string // 空でなければこのディレクトリの .pl を読む
	Players int    // ハートの人数 (0 なら hearts.pl の既定)
	Limit   int    // ハートの終了点
	Games   int    // 大富豪・ナポレオンで通して遊ぶゲーム数
	Seed    int64  // 0 なら毎回変わる
}

// Server はセッションを束ねる。
type Server struct {
	mu       sync.Mutex
	sessions map[string]*session

	plDir   string
	players int
	limit   int
	games   int
	seed    int64
}

// NewServer は設定を受け取ってサーバーを作る。
func NewServer(cfg Config) *Server {
	games := cfg.Games
	if games <= 0 {
		games = 5
	}
	return &Server{
		sessions: map[string]*session{},
		plDir:    cfg.PLDir,
		players:  cfg.Players,
		limit:    cfg.Limit,
		games:    games,
		seed:     cfg.Seed,
	}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", s.showHome)
	s.routesHearts(mux)
	s.routesDaifugo(mux)
	s.routesNapoleon(mux)
}

func (s *Server) routesHearts(mux *http.ServeMux) {
	mux.HandleFunc("GET /hearts", s.showCurrent)
	mux.HandleFunc("GET /hearts/{$}", s.showCurrent)
	mux.HandleFunc("POST /hearts/start", s.postStart)
	mux.HandleFunc("POST /hearts/pass", s.postPass)
	mux.HandleFunc("POST /hearts/play", s.postPlay)
	mux.HandleFunc("POST /hearts/next", s.postNext)
	mux.HandleFunc("POST /hearts/new", s.postNew)
	mux.HandleFunc("POST /hearts/setup", s.postSetup)
	mux.HandleFunc("POST /hearts/preset", s.postPreset)
	mux.HandleFunc("POST /hearts/quit", s.quit(func(sess *session) { sess.game = nil }))

	// JavaScript を切った状態で操作したあとリロードすると、ブラウザは
	// POST 先を GET しに来る。ゲームの状態はサーバーが持っているので、
	// 今の画面へ戻すだけでよい。
	for _, path := range []string{"start", "pass", "play", "next", "new", "setup", "preset", "quit"} {
		mux.HandleFunc("GET /hearts/"+path, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/hearts", http.StatusSeeOther)
		})
	}
}

// =============================================
// ゲームを選ぶ画面
// =============================================

// showHome は遊べるゲームを並べる。対局中のゲームにはその旨を添える。
func (s *Server) showHome(w http.ResponseWriter, r *http.Request) {
	sess := s.session(w, r)
	sess.mu.Lock()
	vm := view.HomeVM{
		HeartsPlaying:   sess.game != nil,
		DaifugoPlaying:  sess.dGame != nil,
		NapoleonPlaying: sess.nGame != nil,
	}
	sess.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := view.HomePage(vm).Render(r.Context(), w); err != nil {
		log.Printf("描画に失敗しました: %v", err)
	}
}

// quit はプレイ中のゲームを打ち切り、ゲームを選ぶ画面へ戻す。
//
// 画面の一部ではなくページごと移る (ヘッダーの「いま遊んでいるゲーム」の
// 印も変わるため)。ボタン側は htmx を通さない普通のフォームにしてあり、
// ここでは "/" へリダイレクトするだけ。アドレス欄も "/" になる。
func (s *Server) quit(clear func(*session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := s.session(w, r)
		sess.mu.Lock()
		clear(sess)
		sess.mu.Unlock()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// =============================================
// セッション
// =============================================

// heartsSession はセッションのハートぶんを用意する。
func (s *Server) heartsSession(sess *session) error {
	if sess.engine != nil {
		return nil
	}
	engine, err := rules.New(s.plDir)
	if err != nil {
		return err
	}
	if s.players != 0 {
		if err := engine.SetPlayers(s.players); err != nil {
			return err
		}
	}
	sess.engine = engine
	sess.opts = rules.DefaultOptions()
	return nil
}

// StartGame は選んだ流儀でハートを 1 局作る。
// 起動時にルールファイルの誤りを弾くためにも使う。
func (s *Server) StartGame(engine *rules.Engine, opts rules.Options) (*game.Game, error) {
	seed := s.seed
	if seed == 0 {
		seed = int64(newRandomID64())
	}
	return game.New(engine, rand.New(rand.NewSource(seed)), s.limit, opts)
}

// session は Cookie からセッションを引く。無ければ作る。
func (s *Server) session(w http.ResponseWriter, r *http.Request) *session {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		sess, ok := s.sessions[c.Value]
		s.mu.Unlock()
		if ok {
			return sess
		}
	}

	id := newSessionID()
	sess := &session{shortID: id[:6]}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return sess
}

// lockedSession はセッションを引いてロックし、prepare でそのゲームの
// Prolog を (まだなら) 読み込む。false が返ったときは応答済みなので
// 呼び出し側は何もしなくてよい。ロックの解除は呼び出し側の責任。
func (s *Server) lockedSession(w http.ResponseWriter, r *http.Request, prepare func(*session) error) (*session, bool) {
	sess := s.session(w, r)
	sess.mu.Lock()
	if err := prepare(sess); err != nil {
		sess.mu.Unlock()
		s.fail(w, err)
		return nil, false
	}
	return sess, true
}

func newSessionID() string {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		// crypto/rand が読めない状況はプロセスを続ける意味が無い。
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// newRandomID64 は配札シャッフルの種を作る。
func newRandomID64() uint64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}

// =============================================
// ハンドラ (ハート)
// =============================================

// showCurrent はゲーム中なら盤面を、開始前ならルール選択の画面を出す。
func (s *Server) showCurrent(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(*game.Game) error { return nil })
}

// postStart は選んだルールでゲームを始める。
func (s *Server) postStart(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.heartsSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.opts = rules.ParseOptions(
		r.FormValue("points"),
		r.FormValue("lead"),
		r.FormValue("hearts_break"),
	)
	g, err := s.StartGame(sess.engine, sess.opts)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.game = g
	s.render(w, r, sess, "")
}

// postSetup はルール選択の画面に戻る。
func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.heartsSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.game = nil
	s.render(w, r, sess, "")
}

// postPreset はルール選択の画面で、流儀をプリセットの組み合わせに
// 置き換える。ゲームはまだ始めず、選び直した画面を返す。
func (s *Server) postPreset(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.heartsSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	var notice string
	if sess.game == nil {
		name := r.FormValue("preset")
		sess.opts = sess.opts.WithPreset(name)
		for _, p := range rules.Presets() {
			if p.Name == name {
				notice = "「" + p.Label + "」の組み合わせにしました。このまま始めるか、ここから選び直してください。"
			}
		}
	}
	s.render(w, r, sess, notice)
}

func (s *Server) postPass(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *game.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitPass(nums)
	})
}

func (s *Server) postPlay(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *game.Game) error {
		n, err := strconv.Atoi(r.FormValue("card"))
		if err != nil {
			return playerErr("カードの指定が正しくありません。")
		}
		return g.PlayCard(n)
	})
}

func (s *Server) postNext(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *game.Game) error { return g.Next() })
}

// postNew は同じルールでゲームを最初からやり直す。
func (s *Server) postNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.heartsSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	g, err := s.StartGame(sess.engine, sess.opts)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.game = g
	s.render(w, r, sess, "")
}

// handle はセッションを取り出し、action で 1 手進めてから画面を返す。
// action が入力ミスを返した場合はメッセージとして画面に出す。
// まだゲームが始まっていなければ action は呼ばず、ルール選択の画面を出す。
func (s *Server) handle(w http.ResponseWriter, r *http.Request, action func(*game.Game) error) {
	sess, ok := s.lockedSession(w, r, s.heartsSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	var msg string
	if sess.game != nil {
		if err := action(sess.game); err != nil {
			var inputError game.ErrPlayerInput
			if !errors.As(err, &inputError) {
				s.fail(w, err)
				return
			}
			msg = inputError.Msg
		}
	}
	s.render(w, r, sess, msg)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, sess *session, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if sess.game == nil {
		vm, err := buildSetupVM(sess.engine, sess.opts, s.limit, msg)
		if err != nil {
			s.fail(w, err)
			return
		}
		if err := view.SetupPage(vm).Render(r.Context(), w); err != nil {
			log.Printf("描画に失敗しました: %v", err)
		}
		return
	}

	vm, err := buildTableVM(sess.game, msg)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := view.TablePage(vm).Render(r.Context(), w); err != nil {
		log.Printf("描画に失敗しました: %v", err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	log.Printf("エラー: %v", err)
	http.Error(w, "内部エラーが発生しました", http.StatusInternalServerError)
}

// playerErr は「入力がルール上おかしい」ことを表す。
// 画面上のメッセージとして扱われ、500 にはならない。
func playerErr(msg string) error {
	return game.ErrPlayerInput{Msg: msg}
}

// formInts は同じ名前で複数送られてくるフォーム値を整数として読む。
func formInts(r *http.Request, name string) ([]int, error) {
	if err := r.ParseForm(); err != nil {
		return nil, playerErr("フォームを読み取れませんでした。")
	}
	values := r.Form[name]
	out := make([]int, 0, len(values))
	for _, v := range values {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, playerErr("カードの指定が正しくありません。")
		}
		out = append(out, n)
	}
	return out, nil
}
