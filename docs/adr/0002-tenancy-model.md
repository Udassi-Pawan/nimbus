# ADR 0002: Tenancy Model

## Status
Accepted

## Decision
Organization → Team → Service → Environment → Kubernetes namespace

Namespace naming: `{service-slug}-{environment}`

## Consequences
- RBAC maps team membership to namespace access later
- GitOps paths can follow `{service}/{environment}`
- Preview envs can use `{service-slug}-pr-{id}` without model changes