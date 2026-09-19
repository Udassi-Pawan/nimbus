-- +goose Up
ALTER TABLE service_environments
    ADD COLUMN deployment_status TEXT NOT NULL DEFAULT 'not_deployed',
    ADD COLUMN deployment_image TEXT NOT NULL DEFAULT '',
    ADD COLUMN deployment_replicas_desired INT NOT NULL DEFAULT 0,
    ADD COLUMN deployment_replicas_ready INT NOT NULL DEFAULT 0,
    ADD COLUMN last_deployed_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE service_environments
    DROP COLUMN IF EXISTS last_deployed_at,
    DROP COLUMN IF EXISTS deployment_replicas_ready,
    DROP COLUMN IF EXISTS deployment_replicas_desired,
    DROP COLUMN IF EXISTS deployment_image,
    DROP COLUMN IF EXISTS deployment_status;
