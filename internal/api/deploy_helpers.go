package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/Udassi-Pawan/nimbus/internal/k8s"
	"github.com/Udassi-Pawan/nimbus/internal/models"
)

func defaultStorageSize(templateID string) string {
	if templateID == "redis" {
		return "1Gi"
	}
	return "5Gi"
}

func (s *Server) helmExtraSetsForDeploy(
	ctx context.Context,
	svc models.Service,
	env models.ServiceEnvironment,
	input models.DeployServiceInput,
) ([]string, bool, string, error) {
	if svc.WorkloadType != "stateful" {
		sets, err := s.helmDependencySets(ctx, svc, input.Environment)
		return sets, false, "", err
	}

	storageSize := input.StorageSize
	if storageSize == "" {
		storageSize = env.StorageSize
	}
	if storageSize == "" {
		storageSize = defaultStorageSize(svc.TemplateID)
	}
	storageClass := input.StorageClass
	if storageClass == "" {
		storageClass = env.StorageClass
	}
	if storageClass == "" {
		storageClass = "local-path"
	}

	secretName := env.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-auth", svc.Slug)
	}

	if err := s.ensureStatefulSecret(ctx, svc, env.Namespace, secretName); err != nil {
		return nil, true, "", err
	}

	sets := []string{
		fmt.Sprintf("persistence.size=%s", storageSize),
		fmt.Sprintf("persistence.storageClass=%s", storageClass),
		fmt.Sprintf("auth.secretName=%s", secretName),
		fmt.Sprintf("auth.username=%s", svc.Slug),
		fmt.Sprintf("auth.database=%s", svc.Slug),
	}
	return sets, true, secretName, nil
}

func (s *Server) ensureStatefulSecret(ctx context.Context, svc models.Service, namespace, secretName string) error {
	if err := s.k8s.EnsureNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("ensure namespace: %w", err)
	}

	exists, err := s.k8s.SecretExists(ctx, namespace, secretName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	password, err := k8s.GeneratePassword(24)
	if err != nil {
		return err
	}

	data := map[string]string{"password": password}
	switch svc.TemplateID {
	case "postgres":
		data["username"] = svc.Slug
		data["database"] = svc.Slug
	case "redis":
		// password only
	default:
		return fmt.Errorf("unsupported stateful template %q", svc.TemplateID)
	}

	if err := s.k8s.EnsureOpaqueSecret(ctx, namespace, secretName, data); err != nil {
		return err
	}
	return nil
}

func (s *Server) helmDependencySets(ctx context.Context, svc models.Service, environment string) ([]string, error) {
	deps, err := s.store.ListServiceDependencies(ctx, svc.ID)
	if err != nil {
		return nil, err
	}

	var sets []string
	for _, dep := range deps {
		target, err := s.store.GetService(ctx, dep.DependsOnServiceID)
		if err != nil {
			return nil, err
		}
		targetEnv, err := s.store.GetServiceEnvironmentByName(ctx, target.ID, environment)
		if err != nil {
			continue
		}
		host := fmt.Sprintf("%s.%s.svc.cluster.local", target.Slug, targetEnv.Namespace)
		secretName := targetEnv.SecretName
		if secretName == "" {
			secretName = fmt.Sprintf("%s-auth", target.Slug)
		}

		switch target.TemplateID {
		case "postgres":
			sets = append(sets,
				"dependencies.postgres.enabled=true",
				fmt.Sprintf("dependencies.postgres.host=%s", host),
				"dependencies.postgres.port=5432",
				fmt.Sprintf("dependencies.postgres.database=%s", target.Slug),
				fmt.Sprintf("dependencies.postgres.secretName=%s", secretName),
			)
		case "redis":
			sets = append(sets,
				"dependencies.redis.enabled=true",
				fmt.Sprintf("dependencies.redis.host=%s", host),
				"dependencies.redis.port=6379",
				fmt.Sprintf("dependencies.redis.secretName=%s", secretName),
			)
		}
	}
	return sets, nil
}

func (s *Server) syncDependencySecretsToAppNamespace(
	ctx context.Context,
	svc models.Service,
	appEnv models.ServiceEnvironment,
	environment string,
) error {
	if svc.WorkloadType == "stateful" {
		return nil
	}

	deps, err := s.store.ListServiceDependencies(ctx, svc.ID)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		target, err := s.store.GetService(ctx, dep.DependsOnServiceID)
		if err != nil {
			return err
		}
		if target.TemplateID != "postgres" && target.TemplateID != "redis" {
			continue
		}
		targetEnv, err := s.store.GetServiceEnvironmentByName(ctx, target.ID, environment)
		if err != nil {
			return fmt.Errorf("dependency %s has no %s environment", target.Slug, environment)
		}
		secretName := targetEnv.SecretName
		if secretName == "" {
			secretName = fmt.Sprintf("%s-auth", target.Slug)
		}
		if err := s.k8s.CopySecretToNamespace(ctx, targetEnv.Namespace, appEnv.Namespace, secretName); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) persistDeployStatus(
	ctx context.Context,
	env models.ServiceEnvironment,
	workload k8s.WorkloadStatus,
	image string,
	secretName string,
	storageSize string,
	storageClass string,
) (models.ServiceEnvironment, error) {
	pvcPhase := ""
	if workload.WorkloadKind == "StatefulSet" {
		phase, err := s.k8s.GetPVCPhase(ctx, env.Namespace, k8s.PVCNameForStatefulSet(workload.DeploymentName))
		if err != nil {
			return models.ServiceEnvironment{}, err
		}
		pvcPhase = phase
	}
	return s.store.UpdateEnvironmentDeployment(
		ctx,
		env.ID,
		workload.DeploymentStatus,
		image,
		int(workload.ReplicasDesired),
		int(workload.ReplicasReady),
		storageSize,
		storageClass,
		secretName,
		pvcPhase,
	)
}

func statefulImageRef(templateID string) string {
	switch templateID {
	case "postgres":
		return "postgres:16"
	case "redis":
		return "redis:7"
	default:
		return ""
	}
}

func normalizeStorage(input models.DeployServiceInput, env models.ServiceEnvironment, templateID string) (size, class string) {
	size = strings.TrimSpace(input.StorageSize)
	if size == "" {
		size = env.StorageSize
	}
	if size == "" {
		size = defaultStorageSize(templateID)
	}
	class = strings.TrimSpace(input.StorageClass)
	if class == "" {
		class = env.StorageClass
	}
	if class == "" {
		class = "local-path"
	}
	return size, class
}
