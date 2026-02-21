# PocketBrain Security Model

## Trust Model

| Entity | Trust Level | Rationale |
|--------|-------------|-----------|
| Registered chats | User-trusted | Only explicitly registered 1-on-1 chats are processed |
| Container agents | Sandboxed | Isolated execution environment |
| WhatsApp messages | User input | Potential prompt injection |

## Security Boundaries

### 1. Container Isolation (Primary Boundary)

Agents execute in containers (lightweight Linux VMs), providing:
- **Process isolation** - Container processes cannot affect the host
- **Filesystem isolation** - Only explicitly mounted directories are visible
- **Non-root execution** - Runs as unprivileged `node` user (uid 1000)
- **Ephemeral containers** - Fresh environment per invocation (`--rm`)

This is the primary security boundary. Rather than relying on application-level permission checks, the attack surface is limited by what's mounted.

### 2. Mount Security

**External Allowlist** - Mount permissions stored at `~/.config/pocketbrain/mount-allowlist.json`, which is:
- Outside project root
- Never mounted into containers
- Cannot be modified by agents

**Default Blocked Patterns:**
```
.ssh, .gnupg, .aws, .azure, .gcloud, .kube, .docker,
credentials, .env, .netrc, .npmrc, id_rsa, id_ed25519,
private_key, .secret
```

**Protections:**
- Symlink resolution before validation (prevents traversal attacks)
- Container path validation (rejects `..` and absolute paths)

### 3. Session Isolation

Each registered chat has isolated OpenCode sessions at `data/sessions/{chat}/.opencode/`:
- Chats cannot see other chats' conversation history
- Session data includes full message history and file contents read
- Prevents cross-chat information disclosure

### 4. IPC Authorization

Messages and task operations are verified against chat identity:

| Operation | Result |
|-----------|--------|
| Send message to own chat | ✓ |
| Send message to other chats | ✗ blocked |
| Schedule task for self | ✓ |
| Schedule task for others | ✗ blocked |
| View/manage own tasks | ✓ |
| View/manage other chats' tasks | ✗ blocked |

The IPC watcher determines chat identity from the **directory** the file was written to — not from what the file content claims. This prevents privilege escalation.

### 5. Credential Handling

**Mounted Credentials:**
- OpenCode auth tokens (filtered from `.env`, read-only)

**NOT Mounted:**
- WhatsApp session (`store/auth/`) - host only
- Mount allowlist - external, never mounted
- Any credentials matching blocked patterns

**Credential Filtering:**
Only these environment variables are exposed to containers:
```typescript
const allowedVars = ['OPENCODE_OAUTH_TOKEN', 'OPENCODE_API_KEY'];
```

> **Note:** opencode credentials are mounted so that OpenCode CLI can authenticate when the agent runs. However, this means the agent itself can discover these credentials via Bash or file operations. Ideally, OpenCode CLI would authenticate without exposing credentials to the agent's execution environment, but I couldn't figure this out. **PRs welcome** if you have ideas for credential isolation.

## Security Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                        UNTRUSTED ZONE                             │
│  WhatsApp Messages (potentially malicious)                        │
└────────────────────────────────┬─────────────────────────────────┘
                                 │
                                 ▼ Input escaping, registered-only filter
┌──────────────────────────────────────────────────────────────────┐
│                     HOST PROCESS (TRUSTED)                        │
│  • Message routing                                                │
│  • IPC authorization                                              │
│  • Mount validation (external allowlist)                          │
│  • Container lifecycle                                            │
│  • Credential filtering                                           │
└────────────────────────────────┬─────────────────────────────────┘
                                 │
                                 ▼ Explicit mounts only
┌──────────────────────────────────────────────────────────────────┐
│                CONTAINER (ISOLATED/SANDBOXED)                     │
│  • Agent execution                                                │
│  • Bash commands (sandboxed)                                      │
│  • File operations (limited to mounts)                            │
│  • Network access (unrestricted)                                  │
│  • Cannot modify security config                                  │
└──────────────────────────────────────────────────────────────────┘
```
