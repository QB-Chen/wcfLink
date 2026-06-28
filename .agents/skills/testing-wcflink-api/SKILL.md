---
name: testing-wcflink-api
description: Test wcfLink backend API hardening and local runtime flows. Use when verifying HTTP auth, listen-address protection, settings updates, media-send path restrictions, sensitive metadata redaction, or Phase 4 agent features (custom modes, LLM providers, usage stats, usage limits).
---

# wcfLink API Runtime Testing

Use this skill when a change affects the backend HTTP API, configuration, local server startup, event/log metadata, or security guards. The app can be tested locally without iLink/WeCom credentials for auth/listen/settings/media-path/redaction hardening.

## Devin Secrets Needed

- None for local API hardening and metadata-redaction tests that use `cmd/wcfLink`, `WCFLINK_API_TOKEN`, isolated state dirs, and synthetic SQLite state.
- Real iLink login or WeCom callback/API end-to-end tests may require repo/provider-specific credentials; request them only when the test truly needs external service access.

## Verified local workflow

1. Build the real binary from the repo root:
   ```bash
   go build -o .devin-test-artifacts/wcfLink-test ./cmd/wcfLink
   ```
2. Use isolated persistent test directories under the repo, e.g. `.devin-test-artifacts/state-token`, not OS temp paths if you need artifacts after restarts.
3. Start a token-protected loopback server with controlled config:
   ```bash
   WCFLINK_STATE_DIR=.devin-test-artifacts/state-token \
   WCFLINK_LISTEN_ADDR=127.0.0.1:18081 \
   WCFLINK_API_TOKEN=devin-test-token \
   WCFLINK_MEDIA_SEND_ROOT=.devin-test-artifacts/state-token/media-root \
   WCFLINK_MAX_MEDIA_BYTES=8 \
   .devin-test-artifacts/wcfLink-test
   ```
4. Probe readiness with `curl -fsS http://127.0.0.1:18081/health/live` before running assertions.
5. For API auth assertions, capture status, headers, and body. Treat HTTP header names case-insensitively; Go may serialize `WWW-Authenticate` as `Www-Authenticate`.
6. To verify non-loopback protection, run the binary with `WCFLINK_LISTEN_ADDR=0.0.0.0:<port>` and no `WCFLINK_API_TOKEN`; it should exit non-zero with `WCFLINK_API_TOKEN is required when listen_addr is not loopback`.
7. To verify runtime settings protection without a token, start a separate loopback instance with no `WCFLINK_API_TOKEN`, then POST `/api/settings` with `listen_addr` set to `0.0.0.0:<port>`; expect HTTP 500 and the same token-required error.
8. To exercise `/api/messages/send-media` path validation without real bot credentials, seed the isolated SQLite DB with one connected account and one peer context before calling the endpoint. If the `sqlite3` CLI is unavailable, create a temporary Go helper that imports `modernc.org/sqlite` or uses `internal/store`, then remove the helper after the test.
9. For media path restriction, set the seeded account `base_url` to an unused local port such as `http://127.0.0.1:9`. A correct outside-root rejection should return `file_path must be under media send root` before any upload/network error like `connection refused`.
10. To verify sensitive metadata redaction, seed sentinel values (for example `SECRET_CONTEXT_TOKEN_123`) into DB-only `events.context_token`, `events.raw_json`, and `peer_contexts.context_token`, then query `/api/events` with the API token. The response should include non-sensitive routing/body fields but not `context_token`, `raw_json`, or the sentinel values.
11. To verify inbound raw event persistence, call `store.SaveInboundMessage` from a temporary Go helper with a sentinel `ilink.WeixinMessage.ContextToken`. Direct DB inspection should show `events.context_token` still stores the token while `events.raw_json` omits both the token value and `context_token` field.
12. To verify log redaction, seed or trigger app-style redacted log metadata and query `/api/logs`; the response should include expected routing metadata but no context-token sentinel or `context_token` field.
13. Clean up background server processes with a trap in shell scripts so ports are not left open.

## Phase 4 Agent Feature Testing

To test custom modes, LLM providers, usage statistics, and usage limits, start the server with agent enabled:

```bash
WCFLINK_STATE_DIR=.devin-test-artifacts/state-agent \
WCFLINK_LISTEN_ADDR=127.0.0.1:18082 \
WCFLINK_API_TOKEN=devin-test-token \
WCFLINK_AGENT_ENABLED=true \
WCFLINK_LLM_API_KEY=sk-dummy-test-key \
WCFLINK_LLM_BASE_URL=http://127.0.0.1:9999 \
WCFLINK_LLM_MODEL=test-model \
.devin-test-artifacts/wcfLink-test
```

A dummy LLM base URL is sufficient for CRUD testing of modes/providers/usage (these are pure DB operations). For usage limit testing, also set `WCFLINK_AGENT_DAILY_TOKEN_LIMIT=100` or `WCFLINK_AGENT_MONTHLY_TOKEN_LIMIT=1000`.

### Custom Modes API (`/api/agent/modes/custom`)
- CRUD: POST (create), GET (list/get by ID), PUT (update by ID), DELETE (delete by ID)
- Slug validation: built-in slugs (icemark, market, prd, prototype, support) are rejected with HTTP 409
- Empty slug/name returns HTTP 400
- Note: PUT overwrites all fields (standard REST PUT, not PATCH)

### LLM Providers API (`/api/agent/llm-providers`)
- CRUD: POST (create), GET (list/get by ID), PUT (update by ID), DELETE (delete by ID)
- API key redaction: responses always show `first4***last4` (or `***` for keys <= 8 chars)
- Redacted key preservation: if PUT body contains `***` in api_key, the server preserves the original key from DB
- To verify key preservation, check the SQLite DB directly with Python: `sqlite3.connect('.../wcfLink.db')` then `SELECT api_key FROM llm_providers WHERE id=?`
- Note: the DB file might be `wcfLink.db` (capital L), not `wcflink.db`

### Usage Statistics API (`/api/agent/usage`)
- Query params: `period=daily|monthly`, `user_id=optional`
- To test, seed `token_usage` table directly via Python sqlite3 with known values, then query the API and verify exact totals
- Fields returned: `user_id`, `total_prompt_tokens`, `total_completion_tokens`, `total_tokens`, `request_count`

### Usage Limit Enforcement
- Set `WCFLINK_AGENT_DAILY_TOKEN_LIMIT` env var before starting the server
- Seed `token_usage` records exceeding the limit for a specific user_id
- Send chat via `POST /api/agent/chat` with `{"session_id":"<user_id>","message":"hello"}`
- **Important**: the chat endpoint uses `session_id` as `UserID` for limit checks, NOT a separate `user_id` field (see `httpapi/server.go:284-288`)
- Over-limit response: `"今日 token 用量已达上限（N/M），请明天再试。"`
- Under-limit: proceeds to LLM call (will get connection refused with dummy URL)

## Useful assertions

- `GET /health/live` without token returns HTTP 200 and JSON containing `"ok":true`.
- Protected routes such as `GET /api/accounts` return HTTP 401 and `{"error":"unauthorized"}` for missing/wrong tokens.
- Protected routes accept both `Authorization: Bearer <token>` and `X-WcfLink-Api-Token: <token>`.
- A no-token process cannot switch settings to a non-loopback listen address.
- `send-media` rejects files outside `WCFLINK_MEDIA_SEND_ROOT` before attempting network upload.
- `GET /api/events` must not expose `context_token`, `raw_json`, or sentinel secret values even if those values exist in DB-only columns.
- DB `peer_contexts.context_token` should remain populated for reply functionality.
- `store.SaveInboundMessage` should preserve `events.context_token` while redacting `events.raw_json`.
- Custom mode creation with built-in slug returns HTTP 409.
- LLM provider API responses never contain raw API keys (always redacted).
- Usage limit check blocks over-limit users with Chinese error message before LLM call.
