# Project Changelog

Significant changes, features, and fixes in reverse chronological order.

---

## 2026-04-21

### Instagram channel integration

**Features**

- **Instagram Direct Messaging support** (`internal/channels/instagram/`): Complete Instagram channel implementation with webhook handling, Graph API integration, and message routing.
- **Webhook-based messaging**: Real-time Instagram DM delivery via Facebook Graph API webhooks with deduplication.
- **Instagram Business Account integration**: Supports Facebook Page connected to Instagram Business Account with long-lived access tokens.
- **Media support**: Handles Instagram images, videos, and audio attachments with URL reference processing.
- **Session management**: Per-user isolated sessions with configurable timeout options.
- **Pairing integration**: Full integration with GoClaw's pairing system for secure access control.
- **UI configuration**: Added to channel configuration UI with schema validation (`ui/web/src/pages/channels/channel-schemas.ts`).
- **Gateway integration**: Seamless integration with existing gateway channel management (`cmd/gateway.go`).

**Components**

- `internal/channels/instagram/instagram.go` — Main channel implementation
- `internal/channels/instagram/types.go` — Type definitions and payload structures
- `internal/channels/instagram/graph_client.go` — Facebook Graph API client
- `internal/channels/instagram/webhook_handler.go` — Webhook request handling
- `internal/channels/instagram/message_handler.go` — Inbound message processing
- `internal/channels/instagram/formatter.go` — Outbound message formatting
- `internal/channels/instagram/router.go` — Global webhook routing

**Security**

- Token validation and verification at startup
- Webhook authentication and verification
- Deduplication mechanism preventing 24-hour replay attacks
- Integration with existing allowlist and pairing policies

**Docs**

- `docs/05-channels-messaging.md` — Updated with Instagram channel documentation

---

## 2026-04-19

### TTS: Gemini provider + ProviderCapabilities schema engine

**Features**

- **Gemini TTS provider** (`internal/audio/gemini/`): supports `gemini-2.5-flash-preview-tts` and `gemini-2.5-pro-preview-tts`. 30 prebuilt voices, 70+ languages, multi-speaker mode (up to 2 simultaneous speakers with distinct voices), audio-tag styling, WAV output via PCM-to-WAV conversion.
- **`ProviderCapabilities` schema** (`internal/audio/capabilities.go`): dynamic per-provider param descriptor. Each provider exposes `Capabilities()` returning `[]ParamSchema` (type, range, default, dependsOn conditions, hidden flag) + `CustomFeatures` flags. UI reads `GET /v1/tts/capabilities` and renders param editors without hard-coded field lists.
- **Dual-read TTS storage**: tenant config read from both legacy flat keys (`tts.provider`, `tts.voice_id`, …) and new params blob (`tts.<provider>.params` JSON). Blob wins on conflict. Allows gradual migration; no data loss on downgrade.
- **`VoiceListProvider` interface** refactor: dynamic voice fetching (ElevenLabs, MiniMax) now via `ListVoices(ctx, ListVoicesOptions)` instead of per-provider ad-hoc methods. Unified `audio.Voice` type.
- **`POST /v1/tts/test-connection`**: ephemeral provider creation from request credentials + short synthesis smoke test. Returns `{ success, latency_ms }`. No provider registration; no config mutation. Operator role required.
- **`GET /v1/tts/capabilities`**: returns `ProviderCapabilities` JSON for all registered providers.

**i18n**

- Backend sentinel error keys (`MsgTtsGeminiInvalidVoice`, `MsgTtsGeminiInvalidModel`, `MsgTtsGeminiSpeakerLimit`, `MsgTtsParamOutOfRange`, `MsgTtsParamDependsOn`, `MsgTtsMiniMaxVoicesFailed`) in all 3 catalogs (EN/VI/ZH).
- HTTP 422 responses for Gemini sentinel errors now use `i18n.T(locale, key, args...)` — locale from `Accept-Language` header.
- ~80 param `label`/`help` keys across web + desktop locale files (EN/VI/ZH); parity enforced by `ui/web/src/__tests__/i18n-tts-key-parity.test.ts`.

**Security**

- SSRF guard on `api_base` override for test-connection (`validateProviderURL()`) — blocks `127.0.0.1` / `localhost` / RFC1918 ranges.

**Docs**

- `docs/tts-provider-capabilities.md` — schema reference + per-provider param tables + storage format + "Adding a new provider" checklist.
- `docs/codebase-summary.md` — TTS subsystem section documenting manager, providers, storage, endpoints.
