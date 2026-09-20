package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"passion/server/db"
	"passion/server/token"
)

// authenticatedHandler is a handler that knows who is asking.
type authenticatedHandler func(http.ResponseWriter, *http.Request, db.Authenticated)

// authenticated refuses anything without a live bearer token.
//
// A missing header, a malformed one, an unknown token and an expired one all
// answer the same, because telling them apart tells an attacker which guess
// was closer.
func (s *Server) authenticated(next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			writeUnauthenticated(w)
			return
		}

		who, err := db.AuthenticateByToken(r.Context(), s.pool, token.Hash(raw))
		switch {
		case errors.Is(err, db.ErrNoAuthToken):
			writeUnauthenticated(w)
			return
		case err != nil:
			writeInternal(w, s.log, err)
			return
		}

		s.slide(r, who)
		next(w, r, who)
	}
}

func bearerToken(r *http.Request) (string, bool) {
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || raw == "" {
		return "", false
	}
	return raw, true
}

// slide pushes the expiry forward, but only once a day of its life has gone,
// so reading does not become writing on every request.
//
// A failure here is logged and ignored: the request is still authenticated,
// and the token still has most of a month left.
func (s *Server) slide(r *http.Request, who db.Authenticated) {
	if time.Until(who.TokenExpiresAt) > tokenLife-24*time.Hour {
		return
	}
	if err := db.SlideAuthToken(r.Context(), s.pool, who.TokenID, time.Now().Add(tokenLife)); err != nil {
		s.log.Error("could not slide the token expiry", "err", err)
	}
}
