package main

// server.go — HTTP ハンドラ。
//
// WebSocket もデータベースも使わない。ブラウザからの POST でゲームを 1 手
// 進め、その場で画面を描き直して返すだけ。htmx が #app だけを差し替える。
// 状態はプロセス内のメモリに、Cookie のセッション ID で紐付けて持つ。

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	"hearts/view"
)

const sessionCookie = "hearts_session"

// session は 1 ブラウザ分のゲーム。
//
// Prolog インタプリタはセッションごとに 1 つ持つ。共有すると同時アクセスで
// 状態が混ざるおそれがあるうえ、人数設定 (hearts.pl の set_players/1) や
// 第1トリックの流儀もセッション単位で変えられなくなる。
type session struct {
	mu    sync.Mutex
	rules *Rules
	opts  Options
	game  *Game // nil ならまだルール選択の画面
}

// Server はセッションを束ねる。
type Server struct {
	mu       sync.Mutex
	sessions map[string]*session

	plDir   string
	players int
	limit   int
	seed    int64
}

func NewServer(plDir string, players, limit int, seed int64) *Server {
	return &Server{
		sessions: map[string]*session{},
		plDir:    plDir,
		players:  players,
		limit:    limit,
		seed:     seed,
	}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", s.showCurrent)
	mux.HandleFunc("POST /start", s.postStart)
	mux.HandleFunc("POST /pass", s.postPass)
	mux.HandleFunc("POST /play", s.postPlay)
	mux.HandleFunc("POST /next", s.postNext)
	mux.HandleFunc("POST /new", s.postNew)
	mux.HandleFunc("POST /setup", s.postSetup)

	// JavaScript を切った状態で操作したあとリロードすると、ブラウザは
	// POST 先を GET しに来る。ゲームの状態はサーバーが持っているので、
	// 今の画面へ戻すだけでよい。
	for _, path := range []string{"/start", "/pass", "/play", "/next", "/new", "/setup"} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		})
	}
}

// =============================================
// セッション
// =============================================

// newRules は Prolog を読み込み、人数設定まで済ませる。
func (s *Server) newRules() (*Rules, error) {
	rules, err := NewRules(s.plDir)
	if err != nil {
		return nil, err
	}
	if s.players != 0 {
		if err := rules.SetPlayers(s.players); err != nil {
			return nil, err
		}
	}
	return rules, nil
}

// startGame は選んだ流儀でゲームを 1 つ作る。
func (s *Server) startGame(rules *Rules, opts Options) (*Game, error) {
	seed := s.seed
	if seed == 0 {
		seed = int64(newRandomID64())
	}
	return NewGame(rules, rand.New(rand.NewSource(seed)), s.limit, opts)
}

// session は Cookie からセッションを引く。無ければ作る。
// 新しいセッションはまだゲームを持たず、ルール選択の画面から始まる。
func (s *Server) session(w http.ResponseWriter, r *http.Request) (*session, error) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		sess, ok := s.sessions[c.Value]
		s.mu.Unlock()
		if ok {
			return sess, nil
		}
	}

	rules, err := s.newRules()
	if err != nil {
		return nil, err
	}
	sess := &session{rules: rules, opts: DefaultOptions()}

	id := newSessionID()
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
	return sess, nil
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
// ハンドラ
// =============================================

// showCurrent はゲーム中なら盤面を、開始前ならルール選択の画面を出す。
func (s *Server) showCurrent(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(*Game) error { return nil })
}

// postStart は選んだルールでゲームを始める。
func (s *Server) postStart(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session(w, r)
	if err != nil {
		s.fail(w, err)
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.opts = ParseOptions(
		r.FormValue("points"),
		r.FormValue("lead"),
		r.FormValue("hearts_break"),
	)
	game, err := s.startGame(sess.rules, sess.opts)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.game = game
	s.render(w, r, sess, "")
}

// postSetup はルール選択の画面に戻る。
func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session(w, r)
	if err != nil {
		s.fail(w, err)
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.game = nil
	s.render(w, r, sess, "")
}

func (s *Server) postPass(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitPass(nums)
	})
}

func (s *Server) postPlay(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *Game) error {
		n, err := strconv.Atoi(r.FormValue("card"))
		if err != nil {
			return inputErr("カードの指定が正しくありません。")
		}
		return g.PlayCard(n)
	})
}

func (s *Server) postNext(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, func(g *Game) error { return g.Next() })
}

// postNew は同じルールでゲームを最初からやり直す。
func (s *Server) postNew(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session(w, r)
	if err != nil {
		s.fail(w, err)
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	game, err := s.startGame(sess.rules, sess.opts)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.game = game
	s.render(w, r, sess, "")
}

// handle はセッションを取り出し、action で 1 手進めてから画面を返す。
// action が入力ミスを返した場合はメッセージとして画面に出す。
// まだゲームが始まっていなければ action は呼ばず、ルール選択の画面を出す。
func (s *Server) handle(w http.ResponseWriter, r *http.Request, action func(*Game) error) {
	sess, err := s.session(w, r)
	if err != nil {
		s.fail(w, err)
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	var msg string
	if sess.game != nil {
		if err := action(sess.game); err != nil {
			var inputError ErrPlayerInput
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
		vm, err := buildSetupVM(sess.rules, sess.opts, s.limit)
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

// formInts は同じ名前で複数送られてくるフォーム値を整数として読む。
func formInts(r *http.Request, name string) ([]int, error) {
	if err := r.ParseForm(); err != nil {
		return nil, inputErr("フォームを読み取れませんでした。")
	}
	values := r.Form[name]
	out := make([]int, 0, len(values))
	for _, v := range values {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, inputErr("カードの指定が正しくありません。")
		}
		out = append(out, n)
	}
	return out, nil
}
