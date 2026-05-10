package web

import (
	"context"
	"errors"
	"net/http"
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
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Redirect", "/dashboard")
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.pages.Signup(w, pages.SignupParams{})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		user := &db.User{
			Email:        email,
			PasswordHash: string(hash),
		}
		if err := s.store.DB.Create(user).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.setAuthCookie(w, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Redirect", "/dashboard")
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
