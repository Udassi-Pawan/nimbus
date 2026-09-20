package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Udassi-Pawan/nimbus/internal/auth"
	"github.com/Udassi-Pawan/nimbus/internal/k8s"
	"github.com/Udassi-Pawan/nimbus/internal/models"
	"github.com/Udassi-Pawan/nimbus/internal/store"
	"github.com/Udassi-Pawan/nimbus/internal/templates"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type contextKey string

const userClaimsKey contextKey = "userClaims"

type Server struct {
	store        *store.Store
	auth         *auth.Service
	generatedDir string
	templatesDir string
	k8s          *k8s.Client
}

func NewServer(st *store.Store, authService *auth.Service, generatedDir, templatesDir string, k8sClient *k8s.Client) *Server {
	return &Server{
		store:        st,
		auth:         authService,
		generatedDir: generatedDir,
		templatesDir: templatesDir,
		k8s:          k8sClient,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Post("/api/v1/auth/login", s.handleLogin)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Get("/teams", s.handleListTeams)
		r.Get("/cluster/status", s.handleGetClusterStatus)
		r.Get("/services", s.handleListServices)
		r.Post("/services", s.handleCreateService)
		r.Post("/services/from-template", s.handleCreateFromTemplate)
		r.Post("/services/{id}/deploy", s.handleDeployService)
		r.Post("/services/{id}/sync-deployment", s.handleSyncDeployment)
		r.Get("/services/{id}/network", s.handleGetServiceNetwork)
		r.Post("/services/{id}/connectivity", s.handleCheckConnectivity)
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

func (s *Server) handleCreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	var input models.CreateFromTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if input.Name == "" || input.Slug == "" || input.TeamID == "" {
		writeError(w, http.StatusBadRequest, "name, slug, and team_id are required")
		return
	}

	templateID := input.TemplateID
	if templateID == "" {
		templateID = "go-api"
	}

	if templateID != "go-api" {
		writeError(w, http.StatusBadRequest, "unsupported template_id")
		return
	}

	generated, err := templates.GenerateGoAPI(s.generatedDir, s.templatesDir, templates.ServiceTemplateInput{
		Name:          input.Name,
		Slug:          input.Slug,
		Description:   input.Description,
		RepositoryURL: input.RepositoryURL,
		OwnerEmail:    input.OwnerEmail,
	})
	if err != nil {
		slog.Error("template generation failed", "error", err, "templates_dir", s.templatesDir)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate template files: %v", err))
		return
	}

	svc, err := s.store.CreateService(r.Context(), models.CreateServiceInput{
		TeamID:        input.TeamID,
		Name:          input.Name,
		Slug:          input.Slug,
		Description:   input.Description,
		RepositoryURL: input.RepositoryURL,
		OwnerEmail:    input.OwnerEmail,
	})
	if err != nil {
		if err == store.ErrDuplicateSlug {
			writeError(w, http.StatusConflict, "service slug already exists for team")
			return
		}
		slog.Error("create service from template failed", "error", err, "slug", input.Slug)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create service: %v", err))
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.template.create", "service", svc.ID, map[string]any{
			"template_id":    templateID,
			"generated_path": generated.OutputPath,
			"slug":           svc.Slug,
		})
	}

	writeJSON(w, http.StatusCreated, models.CreateFromTemplateResponse{
		Service:        svc,
		GeneratedPath:  generated.OutputPath,
		GeneratedFiles: generated.Files,
		TemplateID:     generated.TemplateID,
	})
}

func (s *Server) handleGetClusterStatus(w http.ResponseWriter, r *http.Request) {
	if s.k8s == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes client not configured")
		return
	}

	status, err := s.k8s.Status(r.Context())
	if err != nil {
		slog.Error("cluster status failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, fmt.Sprintf("cluster unreachable: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleGetServiceNetwork(w http.ResponseWriter, r *http.Request) {
	if s.k8s == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes client not configured")
		return
	}

	serviceID := chi.URLParam(r, "id")
	environment := r.URL.Query().Get("environment")
	if environment == "" {
		writeError(w, http.StatusBadRequest, "environment query parameter is required")
		return
	}

	svc, err := s.store.GetService(r.Context(), serviceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service")
		return
	}

	env, err := s.store.GetServiceEnvironmentByName(r.Context(), serviceID, environment)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "environment not found for service")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get environment")
		return
	}

	net, err := s.k8s.GetServiceNetwork(r.Context(), env.Namespace, svc.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("network lookup failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, models.ServiceNetworkResponse{
		Environment:    environment,
		Namespace:      net.Namespace,
		ServiceName:    net.ServiceName,
		Port:           net.Port,
		ClusterIP:      net.ClusterIP,
		DNSShort:       net.DNSShort,
		DNSFQDN:        net.DNSFQDN,
		IngressHost:    net.IngressHost,
		IngressURL:     net.IngressURL,
		EndpointsReady: net.EndpointsReady,
		ServiceFound:   net.ServiceFound,
	})
}

func (s *Server) handleCheckConnectivity(w http.ResponseWriter, r *http.Request) {
	if s.k8s == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes client not configured")
		return
	}

	sourceID := chi.URLParam(r, "id")

	var input models.ConnectivityCheckInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if input.Environment == "" || input.TargetServiceID == "" {
		writeError(w, http.StatusBadRequest, "environment and target_service_id are required")
		return
	}

	sourceSvc, err := s.store.GetService(r.Context(), sourceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service")
		return
	}

	sourceEnv, err := s.store.GetServiceEnvironmentByName(r.Context(), sourceID, input.Environment)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "environment not found for source service")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get source environment")
		return
	}

	targetSvc, err := s.store.GetService(r.Context(), input.TargetServiceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "target service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get target service")
		return
	}

	targetEnv, err := s.store.GetServiceEnvironmentByName(r.Context(), input.TargetServiceID, input.Environment)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "environment not found for target service")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get target environment")
		return
	}

	result, err := s.k8s.CheckConnectivity(
		r.Context(),
		sourceEnv.Namespace,
		targetEnv.Namespace,
		targetSvc.Slug,
		"/health",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connectivity check failed: %v", err))
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.connectivity.check", "service", sourceSvc.ID, map[string]any{
			"environment":         input.Environment,
			"target_service_id":   targetSvc.ID,
			"target_slug":         targetSvc.Slug,
			"ok":                  result.OK,
			"target_url":          result.TargetURL,
		})
	}

	writeJSON(w, http.StatusOK, models.ConnectivityCheckResponse{
		Environment:       input.Environment,
		SourceServiceID:     sourceSvc.ID,
		TargetServiceID:     targetSvc.ID,
		TargetServiceSlug:   targetSvc.Slug,
		OK:                  result.OK,
		Message:             result.Message,
		SourceNamespace:     result.SourceNamespace,
		TargetNamespace:     result.TargetNamespace,
		TargetServiceName:   result.TargetServiceName,
		TargetURL:           result.TargetURL,
		EndpointsReady:      result.EndpointsReady,
	})
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

func (s *Server) handleDeployService(w http.ResponseWriter, r *http.Request) {
	if s.k8s == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes client not configured")
		return
	}

	serviceID := chi.URLParam(r, "id")

	var input models.DeployServiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if input.Environment == "" {
		writeError(w, http.StatusBadRequest, "environment is required")
		return
	}

	svc, err := s.store.GetService(r.Context(), serviceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service")
		return
	}

	env, err := s.store.GetServiceEnvironmentByName(r.Context(), serviceID, input.Environment)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "environment not found for service")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get environment")
		return
	}

	image := input.Image
	if image == "" {
		image = fmt.Sprintf("%s:latest", svc.Slug)
	}

	chartPath := filepath.Join(s.generatedDir, svc.Slug, "helm")
	if _, err := os.Stat(chartPath); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf(
			"no helm chart at %s — create this service from the golden path first",
			chartPath,
		))
		return
	}

	if err := s.k8s.DeployHelm(r.Context(), k8s.HelmDeployParams{
		ReleaseName: svc.Slug,
		ChartPath:   chartPath,
		Namespace:   env.Namespace,
		Image:         image,
		PullPolicy:    "Never",
	}); err != nil {
		slog.Error("helm deploy failed", "error", err, "chart", chartPath, "namespace", env.Namespace)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("deploy failed: %v", err))
		return
	}

	workload, err := s.k8s.GetWorkloadStatus(r.Context(), env.Namespace, svc.Slug)
	if err != nil {
		slog.Error("deploy failed", "error", err, "namespace", env.Namespace, "image", image)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("deploy failed: %v", err))
		return
	}

	updatedEnv, err := s.store.UpdateEnvironmentDeployment(
		r.Context(),
		env.ID,
		workload.DeploymentStatus,
		image,
		int(workload.ReplicasDesired),
		int(workload.ReplicasReady),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deployment status")
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.deploy", "service", svc.ID, map[string]any{
			"environment": input.Environment,
			"namespace":   env.Namespace,
			"image":       image,
			"status":      workload.DeploymentStatus,
			"method":      "helm",
			"chart_path":  chartPath,
		})
	}

	writeJSON(w, http.StatusOK, models.DeployServiceResponse{
		Environment: updatedEnv,
		Workload: models.WorkloadSummary{
			Namespace:         workload.Namespace,
			DeploymentName:    workload.DeploymentName,
			Image:               workload.Image,
			ReplicasDesired:     workload.ReplicasDesired,
			ReplicasReady:       workload.ReplicasReady,
			AvailableReplicas:   workload.AvailableReplicas,
			DeploymentStatus:    workload.DeploymentStatus,
		},
	})
}

func (s *Server) handleSyncDeployment(w http.ResponseWriter, r *http.Request) {
	if s.k8s == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes client not configured")
		return
	}

	serviceID := chi.URLParam(r, "id")

	var input models.SyncDeploymentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if input.Environment == "" {
		writeError(w, http.StatusBadRequest, "environment is required")
		return
	}

	svc, err := s.store.GetService(r.Context(), serviceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service")
		return
	}

	env, err := s.store.GetServiceEnvironmentByName(r.Context(), serviceID, input.Environment)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "environment not found for service")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get environment")
		return
	}

	workload, err := s.k8s.GetWorkloadStatus(r.Context(), env.Namespace, svc.Slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "no deployment found in cluster for this environment")
		return
	}

	image := workload.Image
	if image == "" {
		image = env.DeploymentImage
	}

	updatedEnv, err := s.store.UpdateEnvironmentDeployment(
		r.Context(),
		env.ID,
		workload.DeploymentStatus,
		image,
		int(workload.ReplicasDesired),
		int(workload.ReplicasReady),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deployment status")
		return
	}

	writeJSON(w, http.StatusOK, models.DeployServiceResponse{
		Environment: updatedEnv,
		Workload: models.WorkloadSummary{
			Namespace:         workload.Namespace,
			DeploymentName:    workload.DeploymentName,
			Image:               workload.Image,
			ReplicasDesired:     workload.ReplicasDesired,
			ReplicasReady:       workload.ReplicasReady,
			AvailableReplicas:   workload.AvailableReplicas,
			DeploymentStatus:    workload.DeploymentStatus,
		},
	})
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAuditLogs(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}