package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"passion/store"
)

const authCookieName = "passion_auth"

type ctxKey int

const (
	ctxActor ctxKey = iota
	ctxRequestID
)

// Actor is who the request is acting as. It exists so a store call cannot be made without
// naming someone: every scoped method takes one, and the only way to get one is from a
// verified cookie.
type Actor struct {
	ID    int64
	Email string
}

type claims struct {
	jwt.RegisteredClaims
	// Epoch is compared against the account's token_epoch on every request. A password
	// change bumps the account's, which invalidates every token issued before it — the one
	// thing a stateless token cannot otherwise do.
	Epoch int `json:"epoch"`
}

func actorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxActor).(Actor)
	return a, ok
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestID).(string)
	return id
}

// requestID gives every request a short id, which appears in the log line and on the error
// page, so a user can quote it and it can be found.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()[:8]
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status, w.wrote = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status, w.wrote = http.StatusOK, true
	}
	return w.ResponseWriter.Write(b)
}

// logRequests writes one line per request. A 4xx logs no body; a 5xx logs the error with
// the request id.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFrom(r.Context()),
		}
		if a, ok := actorFrom(r.Context()); ok {
			attrs = append(attrs, "account_id", a.ID)
		}
		if sw.status >= 500 {
			s.log.Error("request failed", attrs...)
		} else {
			s.log.Info("request", attrs...)
		}
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic",
					"error", fmt.Sprint(v),
					"path", r.URL.Path,
					"request_id", requestIDFrom(r.Context()),
					"stack", string(debug.Stack()))
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// checkCSRF rejects a cross-site write. Origin is compared against the request's own host,
// which needs no token and therefore no server-side state.
//
// A missing Origin is allowed: a same-site form post omits it, and SameSite=Lax on the
// cookie already stops the cross-site case for those.
func (s *Server) checkCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "same-origin" || site == "none" {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			s.log.Warn("cross-site write rejected",
				"origin", origin, "host", r.Host, "path", r.URL.Path)
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withActor reads the cookie and puts an Actor in the context when it verifies. It does
// not reject anything: requireActor does that, so a page can be public and still know who
// is looking.
func (s *Server) withActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a, ok := s.actorFromRequest(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), ctxActor, a))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := actorFrom(r.Context()); !ok {
			next := r.URL.RequestURI()
			http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) actorFromRequest(r *http.Request) (Actor, bool) {
	if s.cfg.Auth.DevAuthBypass {
		// Dev only, and refused at boot against Postgres. Resolves the first account
		// rather than a hard-coded id, because identity columns make no promise about 1.
		a, err := s.store.FirstAccount(r.Context())
		if err != nil {
			return Actor{}, false
		}
		return Actor{ID: a.ID, Email: a.Email}, true
	}

	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return Actor{}, false
	}

	var c claims
	token, err := jwt.ParseWithClaims(cookie.Value, &c, func(t *jwt.Token) (any, error) {
		// Pin the algorithm. Without this check a token could name "none" and verify.
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return []byte(s.cfg.Auth.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return Actor{}, false
	}

	id, err := idFromSubject(c.Subject)
	if err != nil {
		return Actor{}, false
	}
	acct, err := s.store.AccountByID(r.Context(), id)
	if err != nil {
		return Actor{}, false
	}
	// The epoch check. A token issued before a password change carries the old epoch and
	// stops here.
	if c.Epoch != acct.TokenEpoch {
		return Actor{}, false
	}
	return Actor{ID: acct.ID, Email: acct.Email}, true
}

func idFromSubject(sub string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(strings.TrimSpace(sub), "%d", &id); err != nil || id <= 0 {
		return 0, errors.New("web: bad subject claim")
	}
	return id, nil
}

// setAuthCookie issues the token. The contract in one place: HttpOnly so script cannot
// read it, SameSite=Lax so a cross-site form post does not carry it, Secure unless
// explicitly disabled for local HTTP, and an expiry that matches the token's.
func (s *Server) setAuthCookie(w http.ResponseWriter, a store.Account) error {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(a.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.TTL())),
		},
		Epoch: a.TokenEpoch,
	})
	signed, err := tok.SignedString([]byte(s.cfg.Auth.JWTSecret))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.cfg.Auth.InsecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.TTL().Seconds()),
	})
	return nil
}

func (s *Server) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.cfg.Auth.InsecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
