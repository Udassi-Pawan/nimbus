package store

const serviceSelectColumns = `
	id, team_id, name, slug, description, repository_url, owner_email,
	template_id, workload_type, created_at, updated_at`

const envSelectColumns = `
	id, service_id, name, namespace,
	deployment_status, deployment_image,
	deployment_replicas_desired, deployment_replicas_ready,
	storage_size, storage_class, secret_name, pvc_phase,
	last_deployed_at, created_at`

const envReturningColumns = envSelectColumns
