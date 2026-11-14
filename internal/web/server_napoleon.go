package web

// server_napoleon.go — ナポレオンの HTTP ハンドラ。

import (
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	ngame "hearts/internal/game/napoleon"
	nrules "hearts/internal/rules/napoleon"
	"hearts/internal/web/view"
)

func (s *Server) routesNapoleon(mux *http.ServeMux) {
	mux.HandleFunc("GET /napoleon", s.showNapoleon)
	mux.HandleFunc("GET /napoleon/{$}", s.showNapoleon)
	mux.HandleFunc("POST /napoleon/start", s.postNapoleonStart)
	mux.HandleFunc("POST /napoleon/setup", s.postNapoleonSetup)
	mux.HandleFunc("POST /napoleon/preset", s.postNapoleonPreset)
	mux.HandleFunc("POST /napoleon/quit", s.quit(func(sess *session) { sess.nGame = nil }))
	mux.HandleFunc("POST /napoleon/new", s.postNapoleonNew)
	mux.HandleFunc("POST /napoleon/bid", s.handleNapoleon(func(r *http.Request, g *ngame.Game) error {
		count, suit, pass, err := parseBid(r.FormValue("bid"))
		if err != nil {
			return err
		}
		return g.SubmitBid(count, suit, pass)
	}))
	mux.HandleFunc("POST /napoleon/aide", s.handleNapoleon(func(r *http.Request, g *ngame.Game) error {
		suit, rank, ok := strings.Cut(r.FormValue("card"), ":")
		if !ok {
			return playerErrNapoleon("カードの指定が正しくありません。")
		}
		return g.SubmitAide(suit, rank)
	}))
	mux.HandleFunc("POST /napoleon/discard", s.handleNapoleon(func(r *http.Request, g *ngame.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitDiscard(nums)
	}))
	mux.HandleFunc("POST /napoleon/play", s.handleNapoleon(func(r *http.Request, g *ngame.Game) error {
		n, err := strconv.Atoi(r.FormValue("card"))
		if err != nil {
			return playerErrNapoleon("カードの指定が正しくありません。")
		}
		return g.PlayCard(n)
	}))
	mux.HandleFunc("POST /napoleon/next", s.handleNapoleon(func(_ *http.Request, g *ngame.Game) error {
		return g.Next()
	}))

	for _, path := range []string{"start", "setup", "preset", "quit", "new", "bid", "aide", "discard", "play", "next"} {
		mux.HandleFunc("GET /napoleon/"+path, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/napoleon", http.StatusSeeOther)
		})
	}
}

// parseBid はフォームの値 ("pass" か "s:13") を読む。
func parseBid(value string) (count int, suit string, pass bool, err error) {
	if value == "pass" || value == "" {
		return 0, "", true, nil
	}
	suit, countText, ok := strings.Cut(value, ":")
	if !ok {
		return 0, "", false, playerErrNapoleon("宣言の指定が正しくありません。")
	}
	n, convErr := strconv.Atoi(countText)
	if convErr != nil {
		return 0, "", false, playerErrNapoleon("宣言の指定が正しくありません。")
	}
	return n, suit, false, nil
}

func playerErrNapoleon(msg string) error { return ngame.ErrPlayerInput{Msg: msg} }

// napoleonSession はセッションのナポレオンぶんを用意する。Prolog は初回に読み込む。
func (s *Server) napoleonSession(sess *session) error {
	if sess.nEngine != nil {
		return nil
	}
	engine, err := nrules.New(s.plDir)
	if err != nil {
		return err
	}
	sess.nEngine = engine
	sess.nOpts = nrules.DefaultOptions()
	return nil
}

func (s *Server) startNapoleon(sess *session) (*ngame.Game, error) {
	seed := s.seed
	if seed == 0 {
		seed = int64(newRandomID64())
	}
	return ngame.New(sess.nEngine, rand.New(rand.NewSource(seed)), s.games, sess.nOpts)
}

func (s *Server) showNapoleon(w http.ResponseWriter, r *http.Request) {
	s.handleNapoleon(func(*http.Request, *ngame.Game) error { return nil })(w, r)
}

func (s *Server) postNapoleonStart(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.napoleonSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.nOpts = nrules.ParseOptions(r.FormValue)
	g, err := s.startNapoleon(sess)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.nGame = g
	s.renderNapoleon(w, r, sess, "")
}

func (s *Server) postNapoleonSetup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.napoleonSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.nGame = nil
	s.renderNapoleon(w, r, sess, "")
}

// postNapoleonPreset はルール選択の画面で、ローカルルールをプリセットの
// 組み合わせに置き換える。ゲームはまだ始めず、選び直した画面を返す。
// 人数は今の選択を残す。
func (s *Server) postNapoleonPreset(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.napoleonSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	var notice string
	if sess.nGame == nil {
		name := r.FormValue("preset")
		sess.nOpts = sess.nOpts.WithPreset(name)
		for _, p := range nrules.Presets() {
			if p.Name == name {
				notice = "「" + p.Label + "」の組み合わせにしました。このまま始めるか、ここから選び直してください。"
			}
		}
	}
	s.renderNapoleon(w, r, sess, notice)
}

func (s *Server) postNapoleonNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.napoleonSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	g, err := s.startNapoleon(sess)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.nGame = g
	s.renderNapoleon(w, r, sess, "")
}

func (s *Server) handleNapoleon(action func(*http.Request, *ngame.Game) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.lockedSession(w, r, s.napoleonSession)
		if !ok {
			return
		}
		defer sess.mu.Unlock()

		var msg string
		if sess.nGame != nil {
			if err := action(r, sess.nGame); err != nil {
				var inputError ngame.ErrPlayerInput
				if !errors.As(err, &inputError) {
					s.fail(w, err)
					return
				}
				msg = inputError.Msg
			}
		}
		s.renderNapoleon(w, r, sess, msg)
	}
}

func (s *Server) renderNapoleon(w http.ResponseWriter, r *http.Request, sess *session, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if sess.nGame == nil {
		vm := buildNapoleonSetupVM(sess.nOpts, s.games, msg)
		if err := view.NapoleonSetupPage(vm).Render(r.Context(), w); err != nil {
			log.Printf("描画に失敗しました: %v", err)
		}
		return
	}

	vm, err := buildNapoleonTableVM(sess.nGame, msg)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := view.NapoleonTablePage(vm).Render(r.Context(), w); err != nil {
		log.Printf("描画に失敗しました: %v", err)
	}
}
