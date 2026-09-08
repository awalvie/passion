package web

import (
	"errors"
	"net/http"
	"strings"

	"passion/store"
)

// authParams renders login.html and signup.html. Both read the same four fields, so they
// share one struct rather than having two that differ by nothing.
//
// InviteRequired is always false: signup is open. The field exists because the shipped
// template still guards an invite block with it, and leaving that block unrendered is
// cheaper than editing a template the old stack also serves.
type authParams struct {
	BaseParams
	AuthFormError  string
	Email          string
	InviteCode     string
	InviteRequired bool
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := actorFrom(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render.page(w, http.StatusOK, "login", authParams{
		BaseParams: BaseParams{Title: "Log in", IsAuthPage: true},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	acct, err := s.store.Authenticate(r.Context(), email, password)
	if err != nil {
		if !errors.Is(err, store.ErrBadCredentials) {
			s.log.Error("authenticating", "error", err, "request_id", requestIDFrom(r.Context()))
		}
		// One message for a wrong password and an unknown address, so the form cannot be
		// used to find out which emails have accounts.
		s.render.page(w, http.StatusUnauthorized, "login", authParams{
			BaseParams:    BaseParams{Title: "Log in", IsAuthPage: true},
			AuthFormError: "Email or password is wrong.",
			Email:         email,
		})
		return
	}

	if err := s.setAuthCookie(w, acct); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

func (s *Server) handleSignupForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := actorFrom(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render.page(w, http.StatusOK, "signup", authParams{
		BaseParams: BaseParams{Title: "Sign up", IsAuthPage: true},
	})
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	fail := func(status int, msg string) {
		s.render.page(w, status, "signup", authParams{
			BaseParams:    BaseParams{Title: "Sign up", IsAuthPage: true},
			AuthFormError: msg,
			Email:         email,
		})
	}

	if strings.TrimSpace(email) == "" {
		fail(http.StatusBadRequest, "Enter an email address.")
		return
	}
	if len(password) < 8 {
		fail(http.StatusBadRequest, "Use a password of at least 8 characters.")
		return
	}

	acct, err := s.store.CreateAccount(r.Context(), email, password, s.cfg.Defaults.TimeZone)
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			fail(http.StatusConflict, "That email already has an account.")
			return
		}
		s.serverError(w, r, err)
		return
	}

	if err := s.setAuthCookie(w, acct); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAuthCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	actor, _ := actorFrom(r.Context())

	err := s.store.ChangePassword(r.Context(), actor.ID,
		r.FormValue("current_password"), r.FormValue("new_password"))
	if err != nil {
		status, msg := statusFor(err)
		if status == http.StatusInternalServerError {
			s.serverError(w, r, err)
			return
		}
		http.Error(w, msg, status)
		return
	}

	// The change bumped the account's token epoch, so the cookie in flight is now stale —
	// including this browser's. Re-issue it so changing a password does not log you out of
	// the session you changed it from, while every other session is ended.
	acct, err := s.store.AccountByID(r.Context(), actor.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.setAuthCookie(w, acct); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// safeNext keeps a redirect on this site. An absolute URL or a scheme-relative one would
// let a crafted link bounce someone off-site after a successful login.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	id := requestIDFrom(r.Context())
	s.log.Error("unhandled error", "error", err, "path", r.URL.Path, "request_id", id)
	http.Error(w, "Something went wrong. Reference: "+id, http.StatusInternalServerError)
}
