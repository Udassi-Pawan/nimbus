package models

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type Team struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	ID            string               `json:"id"`
	TeamID        string               `json:"team_id"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Description   string               `json:"description"`
	RepositoryURL string               `json:"repository_url"`
	OwnerEmail    string               `json:"owner_email"`
	TemplateID    string               `json:"template_id"`
	WorkloadType  string               `json:"workload_type"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Environments  []ServiceEnvironment `json:"environments,omitempty"`
	Dependencies  []ServiceDependency  `json:"dependencies,omitempty"`
}

type ServiceEnvironment struct {
	ID                        string     `json:"id"`
	ServiceID                 string     `json:"service_id"`
	Name                      string     `json:"name"`
	Namespace                 string     `json:"namespace"`
	DeploymentStatus          string     `json:"deployment_status"`
	DeploymentImage           string     `json:"deployment_image"`
	DeploymentReplicasDesired int        `json:"deployment_replicas_desired"`
	DeploymentReplicasReady   int        `json:"deployment_replicas_ready"`
	StorageSize               string     `json:"storage_size"`
	StorageClass              string     `json:"storage_class"`
	SecretName                string     `json:"secret_name"`
	PVCPhase                  string     `json:"pvc_phase"`
	LastDeployedAt            *time.Time `json:"last_deployed_at,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
}

type CreateServiceInput struct {
	TeamID        string   `json:"team_id"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description"`
	RepositoryURL string   `json:"repository_url"`
	OwnerEmail    string   `json:"owner_email"`
	TemplateID    string   `json:"template_id"`
	WorkloadType  string   `json:"workload_type"`
	Environments  []string `json:"environments"`
}

type ServiceDependency struct {
	ID                 string    `json:"id"`
	ServiceID          string    `json:"service_id"`
	DependsOnServiceID string    `json:"depends_on_service_id"`
	DependsOnName      string    `json:"depends_on_name,omitempty"`
	DependsOnSlug      string    `json:"depends_on_slug,omitempty"`
	DependsOnTemplate  string    `json:"depends_on_template_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type AddServiceDependencyInput struct {
	DependsOnServiceID string `json:"depends_on_service_id"`
}

type DataConnectionResponse struct {
	Environment  string `json:"environment"`
	TemplateID   string `json:"template_id"`
	Host         string `json:"host"`
	Port         int32  `json:"port"`
	Database     string `json:"database,omitempty"`
	Username     string `json:"username,omitempty"`
	SecretName   string `json:"secret_name"`
	PasswordKey  string `json:"password_key"`
	UsernameKey  string `json:"username_key"`
	DatabaseKey  string `json:"database_key,omitempty"`
	PVCPhase     string `json:"pvc_phase"`
	EndpointsReady int  `json:"endpoints_ready"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type AuditLog struct {
	ID           string         `json:"id"`
	ActorUserID  *string        `json:"actor_user_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type CreateFromTemplateInput struct {
	TemplateID    string `json:"template_id"`
	TeamID        string `json:"team_id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	RepositoryURL string `json:"repository_url"`
	OwnerEmail    string `json:"owner_email"`
}

type CreateFromTemplateResponse struct {
	Service        Service        `json:"service"`
	GeneratedPath  string         `json:"generated_path"`
	GeneratedFiles []string       `json:"generated_files"`
	TemplateID     string         `json:"template_id"`
}

type DeployServiceInput struct {
	Environment  string `json:"environment"`
	Image        string `json:"image"`
	StorageSize  string `json:"storage_size"`
	StorageClass string `json:"storage_class"`
}

type SyncDeploymentInput struct {
	Environment string `json:"environment"`
}

type WorkloadSummary struct {
	Namespace         string `json:"namespace"`
	DeploymentName    string `json:"deployment_name"`
	Image             string `json:"image"`
	ReplicasDesired   int32  `json:"replicas_desired"`
	ReplicasReady     int32  `json:"replicas_ready"`
	AvailableReplicas int32  `json:"available_replicas"`
	DeploymentStatus  string `json:"deployment_status"`
}

type DeployServiceResponse struct {
	Environment ServiceEnvironment `json:"environment"`
	Workload    WorkloadSummary    `json:"workload"`
}

type ServiceNetworkResponse struct {
	Environment    string `json:"environment"`
	Namespace      string `json:"namespace"`
	ServiceName    string `json:"service_name"`
	Port           int32  `json:"port"`
	ClusterIP      string `json:"cluster_ip"`
	DNSShort       string `json:"dns_short"`
	DNSFQDN        string `json:"dns_fqdn"`
	IngressHost    string `json:"ingress_host,omitempty"`
	IngressURL     string `json:"ingress_url,omitempty"`
	EndpointsReady int    `json:"endpoints_ready"`
	ServiceFound   bool   `json:"service_found"`
}

type ConnectivityCheckInput struct {
	Environment     string `json:"environment"`
	TargetServiceID string `json:"target_service_id"`
}

type ConnectivityCheckResponse struct {
	Environment       string `json:"environment"`
	SourceServiceID     string `json:"source_service_id"`
	TargetServiceID     string `json:"target_service_id"`
	TargetServiceSlug   string `json:"target_service_slug"`
	OK                  bool   `json:"ok"`
	Message             string `json:"message"`
	SourceNamespace     string `json:"source_namespace"`
	TargetNamespace     string `json:"target_namespace"`
	TargetServiceName   string `json:"target_service_name"`
	TargetURL           string `json:"target_url"`
	EndpointsReady      int    `json:"endpoints_ready"`
}