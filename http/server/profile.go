package web

import (
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	switch r.Method {
	case http.MethodGet:
		s.renderProfilePage(w, r, ownerID, "")
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}

		var user db.User
		if err := s.store.DB.Where("id = ?", ownerID).First(&user).Error; err != nil {
			s.notFound(w)
			return
		}

		heightCm, err := parseOptionalInt(r.FormValue("height_cm"))
		if err != nil {
			s.renderProfilePage(w, r, ownerID, "Height must be a whole number.")
			return
		}
		weightKg, err := parseOptionalFloat(r.FormValue("weight_kg"))
		if err != nil {
			s.renderProfilePage(w, r, ownerID, "Weight must be a number.")
			return
		}
		apeIndexCm, err := parseOptionalInt(r.FormValue("ape_index_cm"))
		if err != nil {
			s.renderProfilePage(w, r, ownerID, "Ape index must be a whole number.")
			return
		}
		maxPullUps, err := parseOptionalInt(r.FormValue("max_pull_ups"))
		if err != nil {
			s.renderProfilePage(w, r, ownerID, "Max pull-ups must be a whole number.")
			return
		}
		maxHangKg, err := parseOptionalFloat(r.FormValue("max_hang_kg"))
		if err != nil {
			s.renderProfilePage(w, r, ownerID, "Max hang must be a number.")
			return
		}

		user.HeightCm = heightCm
		user.WeightKg = weightKg
		user.ApeIndexCm = apeIndexCm
		user.MaxPullUps = maxPullUps
		user.MaxHangKg = maxHangKg
		user.BoulderGrade = strings.TrimSpace(r.FormValue("boulder_grade"))
		user.RouteGrade = strings.TrimSpace(r.FormValue("route_grade"))
		bgs := strings.TrimSpace(r.FormValue("boulder_grade_system"))
		if bgs != "font" && bgs != "v_scale" {
			bgs = "font"
		}
		user.BoulderGradeSystem = bgs
		rgs := strings.TrimSpace(r.FormValue("route_grade_system"))
		if rgs != "french" && rgs != "yds" {
			rgs = "french"
		}
		user.RouteGradeSystem = rgs

		if err := s.store.DB.Save(&user).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) renderProfilePage(w http.ResponseWriter, r *http.Request, ownerID uint, formError string) {
	s.renderProfilePageWith(w, r, ownerID, formError, "", false)
}

func (s *Server) renderProfilePageWith(w http.ResponseWriter, r *http.Request, ownerID uint, formError, passwordError string, passwordSuccess bool) {
	var user db.User
	if err := s.store.DB.Where("id = ?", ownerID).First(&user).Error; err != nil {
		s.notFound(w)
		return
	}

	venues, boards, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.Profile(w, pages.ProfileParams{
		Base:             pages.Base{CurrentUserEmail: user.Email},
		UserProfile:      &user,
		ProfileFormError: formError,
		PasswordError:    passwordError,
		PasswordSuccess:  passwordSuccess,
		Venues:           venues,
		Boards:           boards,
	})
}

// handleProfilePassword serves POST /profile/password — an in-app password change
// for the logged-in user (verify current, set new). There is no email-based reset flow.
func (s *Server) handleProfilePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	var user db.User
	if err := s.store.DB.Where("id = ?", ownerID).First(&user).Error; err != nil {
		s.notFound(w)
		return
	}

	current := r.FormValue("current_password")
	next := r.FormValue("new_password")
	confirm := r.FormValue("new_password_confirm")

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		s.renderProfilePageWith(w, r, ownerID, "", "Current password is incorrect.", false)
		return
	}
	if next != confirm {
		s.renderProfilePageWith(w, r, ownerID, "", "New passwords do not match.", false)
		return
	}
	if len(next) < 8 {
		s.renderProfilePageWith(w, r, ownerID, "", "New password must be at least 8 characters.", false)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	user.PasswordHash = string(hash)
	if err := s.store.DB.Save(&user).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	s.renderProfilePageWith(w, r, ownerID, "", "", true)
}

func parseOptionalInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func parseOptionalFloat(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseFloat(raw, 64)
}
