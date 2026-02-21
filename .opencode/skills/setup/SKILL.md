---
name: setup
description: Run initial NanoClaw setup. Use when user wants to install dependencies, authenticate WhatsApp, register their main channel, or start the background services. Triggers on "setup", "install", "configure nanoclaw", or first-time setup requests.
---

# NanoClaw Setup

Run setup scripts automatically. Only pause when user action is required (WhatsApp authentication, configuration choices). Scripts live in `.opencode/skills/setup/scripts/` and emit structured status blocks to stdout. Verbose logs go to `logs/setup.log`.

**Principle:** When something is broken or missing, fix it. Don't tell the user to go fix it themselves unless it genuinely requires their manual action (e.g. scanning a QR code, pasting a secret token). If a dependency is missing, install it. If a service won't start, diagnose and repair. Ask the user for permission when needed, then do the work.

**UX Note:** Use `AskUserQuestion` for all user-facing questions.

## 1. Check Environment

Run `./.opencode/skills/setup/scripts/01-check-environment.sh` and parse the status block.

- If HAS_AUTH=true â†’ note that WhatsApp auth exists, offer to skip step 5
- If HAS_REGISTERED_GROUPS=true â†’ note existing config, offer to skip or reconfigure
- Record PLATFORM, APPLE_CONTAINER, and DOCKER values for step 3

**If NODE_OK=false:**

Node.js is missing or too old. Ask the user if they'd like you to install it. Offer options based on platform:

- macOS: `brew install node@22` (if brew available) or install nvm then `nvm install 22`
- Linux: `curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y nodejs`, or nvm

If brew/nvm aren't installed, install them first (`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"` for brew, `curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash` for nvm). After installing Node, re-run the environment check to confirm NODE_OK=true.

## 2. Install Dependencies

Run `./.opencode/skills/setup/scripts/02-install-deps.sh` and parse the status block.

**If failed:** Read the tail of `logs/setup.log` to diagnose. Common fixes to try automatically:
1. Delete `node_modules` and ``, then re-run the script
2. If permission errors: suggest running with corrected permissions
3. If specific package fails to build (native modules like better-sqlite3): install build tools (`xcode-select --install` on macOS, `build-essential` on Linux), then retry

Only ask the user for help if multiple retries fail with the same error.

## 3. Container Runtime

### 3a. Choose runtime

Check the preflight results for `APPLE_CONTAINER` and `DOCKER`.

**If APPLE_CONTAINER=installed** (macOS only): Ask the user which runtime they'd like to use â€” Docker (default, cross-platform) or Apple Container (native macOS). If they choose Apple Container, run `/convert-to-apple-container` now before continuing, then skip to 3b.

**If APPLE_CONTAINER=not_found**: Use Docker (the default). Proceed to install/start Docker below.

### 3a-docker. Install Docker

- DOCKER=running â†’ continue to 3b
- DOCKER=installed_not_running â†’ start Docker: `open -a Docker` (macOS) or `sudo systemctl start docker` (Linux). Wait 15s, re-check with `docker info`. If still not running, tell the user Docker is starting up and poll a few more times.
- DOCKER=not_found â†’ **ask the user for confirmation before installing.** Tell them Docker is required for running agents and ask if they'd like you to install it. If confirmed:
  - macOS: install via `brew install --cask docker`, then `open -a Docker` and wait for it to start. If brew not available, direct to Docker Desktop download at https://docker.com/products/docker-desktop
  - Linux: install with `curl -fsSL https://get.docker.com | sh && sudo usermod -aG docker $USER`. Note: user may need to log out/in for group membership.

### 3b. Apple Container conversion gate (if needed)

**If the chosen runtime is Apple Container**, you MUST check whether the source code has already been converted from Docker to Apple Container. Do NOT skip this step. Run:

```bash
grep -q "CONTAINER_RUNTIME_BIN = 'container'" src/container-runtime.ts && echo "ALREADY_CONVERTED" || echo "NEEDS_CONVERSION"
```

**If NEEDS_CONVERSION**, the source code still uses Docker as the runtime. You MUST run the `/convert-to-apple-container` skill NOW, before proceeding to the build step.

**If ALREADY_CONVERTED**, the code already uses Apple Container. Continue to 3c.

**If the chosen runtime is Docker**, no conversion is needed â€” Docker is the default. Continue to 3c.

### 3c. Build and test

Run `./.opencode/skills/setup/scripts/03-setup-container.sh --runtime <chosen>` and parse the status block.

**If BUILD_OK=false:** Read `logs/setup.log` tail for the build error.
- If it's a cache issue (stale layers): run `docker builder prune -f`, then retry.
- If Dockerfile syntax or missing files: diagnose from the log and fix.
- Retry the build script after fixing.

**If TEST_OK=false but BUILD_OK=true:** The image built but won't run. Check logs â€” common cause is runtime not fully started. Wait a moment and retry the test.

## 4. OpenCode Authentication (No Script)

If HAS_ENV=true from step 1, read `.env` and check if it already has `OPENCODE_OAUTH_TOKEN` or `OPENCODE_API_KEY`. If so, confirm with user: "You already have OpenCode credentials configured. Want to keep them or reconfigure?" If keeping, skip to step 5.

AskUserQuestion: OpenCode subscription (Pro/Max) vs opencode API key?

**Subscription:** Tell the user:
1. Open another terminal and run: `OpenCode setup-token`
2. Copy the token it outputs
3. Add it to the `.env` file in the project root: `OPENCODE_OAUTH_TOKEN=<token>`
4. Let me know when done

Do NOT ask the user to paste the token into the chat. Do NOT use AskUserQuestion to collect the token. Just tell them what to do, then wait for confirmation that they've added it to `.env`. Once confirmed, verify the `.env` file has the key.

**API key:** Tell the user to add `OPENCODE_API_KEY=<key>` to the `.env` file in the project root, then let you know when done. Once confirmed, verify the `.env` file has the key.

## 5. WhatsApp Authentication

If HAS_AUTH=true from step 1, confirm with user: "WhatsApp credentials already exist. Want to keep them or re-authenticate?" If keeping, skip to step 6.

AskUserQuestion: QR code in browser (recommended) vs pairing code vs QR code in terminal?

- **QR browser:** Run `./.opencode/skills/setup/scripts/04-auth-whatsapp.sh --method qr-browser` (Bash timeout: 150000ms)
- **Pairing code:** Ask for phone number first (country code, no + or spaces, e.g. 14155551234). Run `./.opencode/skills/setup/scripts/04-auth-whatsapp.sh --method pairing-code --phone NUMBER` (Bash timeout: 150000ms). Display the PAIRING_CODE from the status block with instructions.
- **QR terminal:** Run `./.opencode/skills/setup/scripts/04-auth-whatsapp.sh --method qr-terminal`. Tell user to run `cd PROJECT_PATH && npm run auth` in another terminal. Wait for confirmation.

If AUTH_STATUS=already_authenticated â†’ skip ahead.

**If failed:**
- qr_timeout â†’ QR expired. Automatically re-run the auth script to generate a fresh QR. Tell user a new QR is ready.
- logged_out â†’ Delete `store/auth/` and re-run auth automatically.
- 515 â†’ Stream error during pairing. The auth script handles reconnection, but if it persists, re-run the auth script.
- timeout â†’ Auth took too long. Ask user if they scanned/entered the code, offer to retry.

## 6. Configure Trigger and Channel Type

First, determine the phone number situation. Get the bot's WhatsApp number from `store/auth/creds.json`:
`node -e "const c=require('./store/auth/creds.json');console.log(c.me.id.split(':')[0].split('@')[0])"`

AskUserQuestion: Does the bot share your personal WhatsApp number, or does it have its own dedicated phone number?

AskUserQuestion: What trigger word? (default: Andy). In group chats, messages starting with @TriggerWord go to OpenCode. In the main channel, no prefix needed.

AskUserQuestion: Main channel type? (options depend on phone number setup)

**If bot shares user's number (same phone):**
1. Self-chat (chat with yourself) â€” Recommended. You message yourself and the bot responds.
2. Solo group (just you) â€” A group where you're the only member. Good if you want message history separate from self-chat.

**If bot has its own dedicated phone number:**
1. DM with the bot â€” Recommended. You message the bot's number directly.
2. Solo group with the bot â€” A group with just you and the bot, no one else.

Do NOT show options that don't apply to the user's setup. For example, don't offer "DM with the bot" if the bot shares the user's number (you can't DM yourself on WhatsApp).

## 7. Sync and Select Group (If Group Channel)

**For personal chat:** The JID is the bot's own phone number from step 6. Construct as `NUMBER@s.whatsapp.net`.

**For DM with bot's dedicated number:** Ask for the bot's phone number, construct JID as `NUMBER@s.whatsapp.net`.

**For group (solo or with bot):**
1. Run `./.opencode/skills/setup/scripts/05-sync-groups.sh` (Bash timeout: 60000ms)
2. **If BUILD=failed:** Read `logs/setup.log`, fix the TypeScript error, re-run.
3. **If GROUPS_IN_DB=0:** Check `logs/setup.log` for the sync output. Common causes: WhatsApp auth expired (re-run step 5), connection timeout (re-run sync script with longer timeout).
4. Run `./.opencode/skills/setup/scripts/05b-list-groups.sh` to get groups (pipe-separated JID|name lines). Do NOT display the output to the user.
5. Pick the most likely candidates (e.g. groups with the trigger word or "NanoClaw" in the name, small/solo groups) and present them as AskUserQuestion options â€” show names only, not JIDs. Include an "Other" option if their group isn't listed. If they pick Other, search by name in the DB or re-run with a higher limit.

## 8. Register Channel

Run `./.opencode/skills/setup/scripts/06-register-channel.sh` with args:
- `--jid "JID"` â€” from step 7
- `--name "main"` â€” always "main" for the first channel
- `--trigger "@TriggerWord"` â€” from step 6
- `--folder "main"` â€” always "main" for the first channel
- `--no-trigger-required` â€” if personal chat, DM, or solo group
- `--assistant-name "Name"` â€” if trigger word differs from "Andy"

## 9. Mount Allowlist

AskUserQuestion: Want the agent to access directories outside the NanoClaw project? (Git repos, project folders, documents, etc.)

**If no:** Run `./.opencode/skills/setup/scripts/07-configure-mounts.sh --empty`

**If yes:** Collect directory paths and permissions (read-write vs read-only). Ask about non-main group read-only restriction (recommended: yes). Build the JSON and pipe it to the script:

`echo '{"allowedRoots":[...],"blockedPatterns":[],"nonMainReadOnly":true}' | ./.opencode/skills/setup/scripts/07-configure-mounts.sh`

Tell user how to grant a group access: add `containerConfig.additionalMounts` to their entry in `data/registered_groups.json`.

## 10. Start Service

If the service is already running (check `launchctl list | grep nanoclaw` on macOS), unload it first: `launchctl unload ~/Library/LaunchAgents/com.nanoclaw.plist` â€” then proceed with a clean install.

Run `./.opencode/skills/setup/scripts/08-setup-service.sh` and parse the status block.

**If SERVICE_LOADED=false:**
- Read `logs/setup.log` for the error.
- Common fix: plist already loaded with different path. Unload the old one first, then re-run.
- On macOS: check `launchctl list | grep nanoclaw` to see if it's loaded with an error status. If the PID column is `-` and the status column is non-zero, the service is crashing. Read `logs/nanoclaw.error.log` for the crash reason and fix it (common: wrong Node path, missing .env, missing auth).
- On Linux: check `systemctl --user status nanoclaw` for the error and fix accordingly.
- Re-run the setup-service script after fixing.

## 11. Verify

Run `./.opencode/skills/setup/scripts/09-verify.sh` and parse the status block.

**If STATUS=failed, fix each failing component:**
- SERVICE=stopped â†’ run `npm run build` first, then restart: `launchctl kickstart -k gui/$(id -u)/com.nanoclaw` (macOS) or `systemctl --user restart nanoclaw` (Linux). Re-check.
- SERVICE=not_found â†’ re-run step 10.
- CREDENTIALS=missing â†’ re-run step 4.
- WHATSAPP_AUTH=not_found â†’ re-run step 5.
- REGISTERED_GROUPS=0 â†’ re-run steps 7-8.
- MOUNT_ALLOWLIST=missing â†’ run `./.opencode/skills/setup/scripts/07-configure-mounts.sh --empty` to create a default.

After fixing, re-run `09-verify.sh` to confirm everything passes.

Tell user to test: send a message in their registered chat (with or without trigger depending on channel type).

Show the log tail command: `tail -f logs/nanoclaw.log`

## Troubleshooting

**Service not starting:** Check `logs/nanoclaw.error.log`. Common causes: wrong Node path in plist (re-run step 10), missing `.env` (re-run step 4), missing WhatsApp auth (re-run step 5).

**Container agent fails ("OpenCode CLI process exited with code 1"):** Ensure the container runtime is running â€” start it with the appropriate command for your runtime. Check container logs in `groups/main/logs/container-*.log`.

**No response to messages:** Verify the trigger pattern matches. Main channel and personal/solo chats don't need a prefix. Check the registered JID in the database: `sqlite3 store/messages.db "SELECT * FROM registered_groups"`. Check `logs/nanoclaw.log`.

**Messages sent but not received (DMs):** WhatsApp may use LID (Linked Identity) JIDs. Check logs for LID translation. Verify the registered JID has no device suffix (should be `number@s.whatsapp.net`, not `number:0@s.whatsapp.net`).

**WhatsApp disconnected:** Run `npm run auth` to re-authenticate, then `npm run build && launchctl kickstart -k gui/$(id -u)/com.nanoclaw`.

**Unload service:** `launchctl unload ~/Library/LaunchAgents/com.nanoclaw.plist`


