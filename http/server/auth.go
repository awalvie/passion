package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"passion/db"
	"passion/pages"
)

const authCookieName = "passion_auth"

type authContextKey string

const (
	authUserIDKey    authContextKey = "auth_user_id"
	authUserEmailKey authContextKey = "auth_user_email"
)

type authClaims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.pages.Login(w, pages.LoginParams{})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
		password := r.FormValue("password")
		if email == "" || password == "" {
			s.pages.Login(w, pages.LoginParams{AuthFormError: "Email and password are required."})
			return
		}

		var user db.User
		if err := s.store.DB.Where("email = ?", email).First(&user).Error; err != nil {
			s.pages.Login(w, pages.LoginParams{AuthFormError: "Invalid email or password."})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			s.pages.Login(w, pages.LoginParams{AuthFormError: "Invalid email or password."})
			return
		}

		if err := s.setAuthCookie(w, user.ID); err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/dashboard")
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.pages.Signup(w, pages.SignupParams{})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
		password := r.FormValue("password")
		confirm := r.FormValue("password_confirm")
		if email == "" || password == "" {
			s.pages.Signup(w, pages.SignupParams{AuthFormError: "Email and password are required."})
			return
		}
		if password != confirm {
			s.pages.Signup(w, pages.SignupParams{AuthFormError: "Passwords do not match."})
			return
		}
		if len(password) < 8 {
			s.pages.Signup(w, pages.SignupParams{AuthFormError: "Password must be at least 8 characters."})
			return
		}
		var existing db.User
		if err := s.store.DB.Where("email = ?", email).First(&existing).Error; err == nil {
			s.pages.Signup(w, pages.SignupParams{AuthFormError: "An account with that email already exists."})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		user := &db.User{
			Email:        email,
			PasswordHash: string(hash),
		}
		if err := s.store.DB.Create(user).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		if s.yamlImport != nil {
			opts := *s.yamlImport
			opts.OwnerID = user.ID
			if err := s.store.ImportYAML(opts); err != nil {
				s.logger.Error("yaml import for new user failed", "owner_id", user.ID, "error", err)
			}
		}
		if err := s.setAuthCookie(w, user.ID); err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/dashboard")
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", "/login")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// csrfMiddleware rejects state-changing requests whose Origin doesn't match the
// request host. Safe methods (GET, HEAD, OPTIONS) are always passed through.
// This is a lightweight defence-in-depth layer on top of SameSite=Lax cookies.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header — allow (same-site form submissions omit it).
			next.ServeHTTP(w, r)
			return
		}
		// Compare the Origin host to the request Host.
		parsed, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
			s.logger.Warn("csrf: origin mismatch",
				"origin", origin,
				"host", r.Host,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.devAuthBypass {
			var demo db.User
			if err := s.store.DB.Where("id = ?", 1).First(&demo).Error; err != nil {
				// Auto-create the dev bypass user when it doesn't exist yet.
				demo = db.User{Email: "dev@passion.local", PasswordHash: ""}
				demo.ID = 1
				if err := s.store.DB.Create(&demo).Error; err != nil {
					http.Error(w, "dev auth bypass: failed to create demo user", http.StatusInternalServerError)
					return
				}
			}
			ctx := context.WithValue(r.Context(), authUserIDKey, demo.ID)
			ctx = context.WithValue(ctx, authUserEmailKey, demo.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		userID, userEmail, claims, err := s.userFromRequest(r)
		if err != nil {
			s.unauthorizedRedirect(w, r)
			return
		}
		// Sliding window: refresh cookie when less than half the TTL remains.
		if claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) < s.jwtTTL/2 {
			_ = s.setAuthCookie(w, userID)
		}
		ctx := context.WithValue(r.Context(), authUserIDKey, userID)
		ctx = context.WithValue(ctx, authUserEmailKey, userEmail)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) userFromRequest(r *http.Request) (uint, string, *authClaims, error) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return 0, "", nil, err
	}
	token, err := jwt.ParseWithClaims(cookie.Value, &authClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("invalid signing method")
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*authClaims)
	if !ok || claims.UserID == 0 {
		return 0, "", nil, errors.New("invalid claims")
	}

	var user db.User
	if err := s.store.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return 0, "", nil, err
	}
	return user.ID, user.Email, claims, nil
}

func (s *Server) setAuthCookie(w http.ResponseWriter, userID uint) error {
	now := time.Now()
	claims := authClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    tokenStr,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.jwtTTL.Seconds()),
	})
	return nil
}

func (s *Server) currentUserID(r *http.Request) (uint, bool) {
	v := r.Context().Value(authUserIDKey)
	id, ok := v.(uint)
	if !ok || id == 0 {
		return 0, false
	}
	return id, true
}

// mustUserID returns the authenticated user's ID from context.
// It panics if the auth middleware hasn't run — only call from routes inside the auth group.
func (s *Server) mustUserID(r *http.Request) uint {
	id, ok := s.currentUserID(r)
	if !ok {
		panic("mustUserID called outside auth middleware")
	}
	return id
}

func (s *Server) currentUserEmail(r *http.Request) string {
	v := r.Context().Value(authUserEmailKey)
	email, _ := v.(string)
	return email
}

func (s *Server) unauthorizedRedirect(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("HX-Request")), "true") {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
