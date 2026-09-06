package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Udassi-Pawan/nimbus/internal/auth"
	"github.com/Udassi-Pawan/nimbus/internal/models"
	"github.com/Udassi-Pawan/nimbus/internal/store"
)

type contextKey string

const userClaimsKey contextKey = "userClaims"

type Server struct {
	store *store.Store
	auth  *auth.Service
}

func NewServer(st *store.Store, authService *auth.Service) *Server {
	return &Server{store: st, auth: authService}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Post("/api/v1/auth/login", s.handleLogin)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.authMiddleware)
	
		r.Get("/teams", s.handleListTeams)
		r.Get("/services", s.handleListServices)
		r.Post("/services", s.handleCreateService)
		r.Get("/services/{id}", s.handleGetService)
		r.Get("/audit-logs", s.handleListAuditLogs)
	})

	return r
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := s.auth.ParseToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func claimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*auth.Claims)
	return claims, ok
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input models.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	user, passwordHash, err := s.store.GetUserByEmail(r.Context(), input.Email)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to authenticate")
		return
	}

	if err := s.auth.CheckPassword(passwordHash, input.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.auth.CreateToken(user.ID, user.Email, user.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	actorID := user.ID
	_ = s.store.CreateAuditLog(r.Context(), &actorID, "auth.login", "user", user.ID, nil)

	writeJSON(w, http.StatusOK, models.LoginResponse{Token: token, User: user})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.store.ListTeams(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.store.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	svc, err := s.store.GetService(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var input models.CreateServiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if input.Name == "" || input.Slug == "" || input.TeamID == "" {
		writeError(w, http.StatusBadRequest, "name, slug, and team_id are required")
		return
	}

	svc, err := s.store.CreateService(r.Context(), input)
	if err != nil {
		if err == store.ErrDuplicateSlug {
			writeError(w, http.StatusConflict, "service slug already exists for team")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create service")
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.create", "service", svc.ID, map[string]any{
			"name": svc.Name,
			"slug": svc.Slug,
		})
	}

	writeJSON(w, http.StatusCreated, svc)
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAuditLogs(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}