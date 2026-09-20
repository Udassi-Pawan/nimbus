-- +goose Up
ALTER TABLE services
    ADD COLUMN template_id TEXT NOT NULL DEFAULT 'go-api',
    ADD COLUMN workload_type TEXT NOT NULL DEFAULT 'stateless';

ALTER TABLE service_environments
    ADD COLUMN storage_size TEXT NOT NULL DEFAULT '',
    ADD COLUMN storage_class TEXT NOT NULL DEFAULT '',
    ADD COLUMN secret_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN pvc_phase TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE service_environments
    DROP COLUMN IF EXISTS pvc_phase,
    DROP COLUMN IF EXISTS secret_name,
    DROP COLUMN IF EXISTS storage_class,
    DROP COLUMN IF EXISTS storage_size;

ALTER TABLE services
    DROP COLUMN IF EXISTS workload_type,
    DROP COLUMN IF EXISTS template_id;
