package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Udassi-Pawan/nimbus/internal/k8s"
	"github.com/Udassi-Pawan/nimbus/internal/models"
	"github.com/Udassi-Pawan/nimbus/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListDependencies(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "id")
	deps, err := s.store.ListServiceDependencies(r.Context(), serviceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list dependencies")
		return
	}
	writeJSON(w, http.StatusOK, deps)
}

func (s *Server) handleAddDependency(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "id")

	var input models.AddServiceDependencyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if input.DependsOnServiceID == "" {
		writeError(w, http.StatusBadRequest, "depends_on_service_id is required")
		return
	}

	dep, err := s.store.AddServiceDependency(r.Context(), serviceID, input.DependsOnServiceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "service or dependency target not found")
			return
		}
		if err == store.ErrDuplicateSlug {
			writeError(w, http.StatusConflict, "dependency already exists")
			return
		}
		if err.Error() == "service cannot depend on itself" {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add dependency")
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.dependency.add", "service", serviceID, map[string]any{
			"depends_on_service_id": input.DependsOnServiceID,
		})
	}

	writeJSON(w, http.StatusCreated, dep)
}

func (s *Server) handleDeleteDependency(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "id")
	depID := chi.URLParam(r, "depId")

	if err := s.store.DeleteServiceDependency(r.Context(), serviceID, depID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "dependency not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete dependency")
		return
	}

	if claims, ok := claimsFromContext(r.Context()); ok {
		actorID := claims.UserID
		_ = s.store.CreateAuditLog(r.Context(), &actorID, "service.dependency.remove", "service", serviceID, map[string]any{
			"dependency_id": depID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetDataConnection(w http.ResponseWriter, r *http.Request) {
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
	if svc.WorkloadType != "stateful" {
		writeError(w, http.StatusBadRequest, "data connection is only available for stateful services")
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

	port := int32(5432)
	passwordKey := "password"
	usernameKey := "username"
	databaseKey := "database"
	if svc.TemplateID == "redis" {
		port = 6379
		usernameKey = ""
		databaseKey = ""
	}

	secretName := env.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-auth", svc.Slug)
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Slug, env.Namespace)
	net, _ := s.k8s.GetServiceNetwork(r.Context(), env.Namespace, svc.Slug)
	endpoints := 0
	if net.ServiceFound {
		endpoints = net.EndpointsReady
		if net.Port > 0 {
			port = net.Port
		}
	}

	pvcPhase := env.PVCPhase
	if pvcPhase == "" {
		pvcPhase, _ = s.k8s.GetPVCPhase(r.Context(), env.Namespace, k8s.PVCNameForStatefulSet(svc.Slug))
	}

	writeJSON(w, http.StatusOK, models.DataConnectionResponse{
		Environment:    environment,
		TemplateID:     svc.TemplateID,
		Host:           host,
		Port:           port,
		Database:       svc.Slug,
		Username:       svc.Slug,
		SecretName:     secretName,
		PasswordKey:    passwordKey,
		UsernameKey:    usernameKey,
		DatabaseKey:    databaseKey,
		PVCPhase:       pvcPhase,
		EndpointsReady: endpoints,
	})
}
