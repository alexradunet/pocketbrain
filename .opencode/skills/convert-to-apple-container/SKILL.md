---
name: convert-to-apple-container
description: ARCHIVED - This skill was designed for a container-per-invocation architecture that has been superseded. PocketBrain now runs as a single Bun process inside Docker; the Docker container IS the isolation. This skill is no longer applicable.
---

# Convert to Apple Container (Archived)

> **Note**: This skill was designed for an older container-per-invocation architecture where PocketBrain would spawn a new container for each agent session. That architecture has been replaced.
>
> **Current architecture**: PocketBrain runs as a single long-lived Bun process inside a Docker container. The OpenCode SDK (`@opencode-ai/sdk`) manages agent sessions in-process. The Docker container itself provides isolation — there are no per-invocation containers to switch runtimes on.
>
> If you want to switch the Docker container runtime (e.g. from Docker Desktop to OrbStack or Podman on macOS), that's a host-level configuration change, not a code change.

## If you want to use a different container runtime on macOS

PocketBrain uses Docker Compose for building and running the container. The `docker compose` commands in `package.json` work with:

- **Docker Desktop** — the default
- **OrbStack** — drop-in Docker Desktop replacement, faster on Apple Silicon; install OrbStack and it automatically replaces the `docker` CLI
- **Rancher Desktop** — another Docker Desktop alternative with containerd backend

No code changes are needed for any of these — they all implement the Docker CLI interface.

## If you want native macOS container execution (Apple Container framework)

Apple's native container framework (`container` CLI) is incompatible with Docker Compose and uses a different image format. Migrating to it would require:

1. Rewriting `docker-compose.yml` → custom build/run scripts using `container` CLI
2. Rebuilding the image with `container build`
3. Rewriting all `package.json` scripts

This is a significant undertaking and not covered by a simple skill. Consult the Apple Container documentation and plan carefully before attempting it.
