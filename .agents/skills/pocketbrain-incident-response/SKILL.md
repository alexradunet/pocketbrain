---
name: pocketbrain-incident-response
description: Drive first-response diagnosis and recovery for PocketBrain runtime incidents.
compatibility: opencode
metadata:
  audience: operators
  scope: incident
---

## What I do

- Triage incident severity
- Run first-response diagnostics
- Execute scenario-based mitigation and recovery verification

## When to use me

Use this during runtime outages, degraded behavior, or suspected data integrity incidents.

## Canonical references

- Primary workflow: `docs/runbooks/incident-response.md`
- Related recovery/deploy flow: `docs/runbooks/runtime-deploy.md`

## Recovery verification

- both services healthy
- logs stable without repeated exceptions
- one chat command path succeeds
