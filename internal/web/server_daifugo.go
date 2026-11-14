package web

// server_daifugo.go — 大富豪の HTTP ハンドラ。
//
// ハート (server.go) と同じ作りで、POST でゲームを 1 手進め、その場で
// 画面を描き直して返す。状態はセッションに持つ。

import (
	"errors"
	"log"
	"math/rand"
	"net/http"

	dgame "hearts/internal/game/daifugo"
	drules "hearts/internal/rules/daifugo"
	"hearts/internal/web/view"
)

func (s *Server) routesDaifugo(mux *http.ServeMux) {
	mux.HandleFunc("GET /daifugo", s.showDaifugo)
	mux.HandleFunc("GET /daifugo/{$}", s.showDaifugo)
	mux.HandleFunc("POST /daifugo/start", s.postDaifugoStart)
	mux.HandleFunc("POST /daifugo/setup", s.postDaifugoSetup)
	mux.HandleFunc("POST /daifugo/preset", s.postDaifugoPreset)
	mux.HandleFunc("POST /daifugo/quit", s.quit(func(sess *session) { sess.dGame = nil }))
	mux.HandleFunc("POST /daifugo/new", s.postDaifugoNew)
	mux.HandleFunc("POST /daifugo/play", s.handleDaifugo(func(r *http.Request, g *dgame.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.PlayCards(nums)
	}))
	mux.HandleFunc("POST /daifugo/pass", s.handleDaifugo(func(_ *http.Request, g *dgame.Game) error {
		return g.Pass()
	}))
	mux.HandleFunc("POST /daifugo/exchange", s.handleDaifugo(func(r *http.Request, g *dgame.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitExchange(nums)
	}))
	mux.HandleFunc("POST /daifugo/next", s.handleDaifugo(func(_ *http.Request, g *dgame.Game) error {
		return g.Next()
	}))
	mux.HandleFunc("POST /daifugo/jackback", s.handleDaifugo(func(r *http.Request, g *dgame.Game) error {
		return g.SubmitJackBack(r.FormValue("apply") == "yes")
	}))
	mux.HandleFunc("POST /daifugo/ten-discard", s.handleDaifugo(func(r *http.Request, g *dgame.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitTenDiscard(nums)
	}))
	mux.HandleFunc("POST /daifugo/seven-give", s.handleDaifugo(func(r *http.Request, g *dgame.Game) error {
		nums, err := formInts(r, "card")
		if err != nil {
			return err
		}
		return g.SubmitSevenGive(nums)
	}))

	// JavaScript を切った状態で操作したあとリロードすると、ブラウザは
	// POST 先を GET しに来る。今の画面へ戻すだけでよい。
	for _, path := range []string{
		"start", "setup", "preset", "quit", "new", "play", "pass", "exchange", "next",
		"jackback", "ten-discard", "seven-give",
	} {
		mux.HandleFunc("GET /daifugo/"+path, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/daifugo", http.StatusSeeOther)
		})
	}
}

// daifugoSession はセッションの大富豪ぶんを取り出す。Prolog は初回に読み込む。
func (s *Server) daifugoSession(sess *session) error {
	if sess.dEngine != nil {
		return nil
	}
	engine, err := drules.New(s.plDir)
	if err != nil {
		return err
	}
	sess.dEngine = engine
	sess.dOpts = drules.DefaultOptions()
	return nil
}

// startDaifugo は選んだルールでゲームを 1 つ作る。
func (s *Server) startDaifugo(sess *session) (*dgame.Game, error) {
	seed := s.seed
	if seed == 0 {
		seed = int64(newRandomID64())
	}
	g, err := dgame.New(sess.dEngine, rand.New(rand.NewSource(seed)), s.games, sess.dOpts)
	if err != nil {
		return nil, err
	}
	// 大富豪はトリックの区切りが無く、1 回の応答で CPU が何人も続けて手を
	// 出すので、画面に残る直近 10 件からはすぐに流れてしまう。
	// あとから読み返せるように、進行をサーバーのログにも流す。
	g.SetLogger(func(format string, a ...interface{}) {
		log.Printf("大富豪[%s] "+format, append([]interface{}{sess.shortID}, a...)...)
	})
	return g, nil
}

func (s *Server) showDaifugo(w http.ResponseWriter, r *http.Request) {
	s.handleDaifugo(func(*http.Request, *dgame.Game) error { return nil })(w, r)
}

func (s *Server) postDaifugoStart(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.daifugoSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.dOpts = drules.ParseOptions(r.FormValue)
	g, err := s.startDaifugo(sess)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.dGame = g
	s.renderDaifugo(w, r, sess, "")
}

func (s *Server) postDaifugoSetup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.daifugoSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	sess.dGame = nil
	s.renderDaifugo(w, r, sess, "")
}

// postDaifugoPreset はルール選択の画面で、ローカルルールをプリセットの
// 組み合わせに置き換える。ゲームはまだ始めず、選び直した画面を返す。
// 人数は今の選択を残す。
func (s *Server) postDaifugoPreset(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.daifugoSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	var notice string
	if sess.dGame == nil {
		name := r.FormValue("preset")
		sess.dOpts = sess.dOpts.WithPreset(name)
		for _, p := range drules.Presets() {
			if p.Name == name {
				notice = "「" + p.Label + "」の組み合わせにしました。このまま始めるか、ここから選び直してください。"
			}
		}
	}
	s.renderDaifugo(w, r, sess, notice)
}

func (s *Server) postDaifugoNew(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lockedSession(w, r, s.daifugoSession)
	if !ok {
		return
	}
	defer sess.mu.Unlock()

	g, err := s.startDaifugo(sess)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.dGame = g
	s.renderDaifugo(w, r, sess, "")
}

// handleDaifugo はセッションを取り出し、action で 1 手進めてから画面を返す。
func (s *Server) handleDaifugo(action func(*http.Request, *dgame.Game) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.lockedSession(w, r, s.daifugoSession)
		if !ok {
			return
		}
		defer sess.mu.Unlock()

		var msg string
		if sess.dGame != nil {
			if err := action(r, sess.dGame); err != nil {
				var inputError dgame.ErrPlayerInput
				if !errors.As(err, &inputError) {
					s.fail(w, err)
					return
				}
				msg = inputError.Msg
			}
		}
		s.renderDaifugo(w, r, sess, msg)
	}
}

func (s *Server) renderDaifugo(w http.ResponseWriter, r *http.Request, sess *session, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if sess.dGame == nil {
		vm := buildDaifugoSetupVM(sess.dOpts, s.games, msg)
		if err := view.DaifugoSetupPage(vm).Render(r.Context(), w); err != nil {
			log.Printf("描画に失敗しました: %v", err)
		}
		return
	}

	vm, err := buildDaifugoTableVM(sess.dGame, msg)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := view.DaifugoTablePage(vm).Render(r.Context(), w); err != nil {
		log.Printf("描画に失敗しました: %v", err)
	}
}
