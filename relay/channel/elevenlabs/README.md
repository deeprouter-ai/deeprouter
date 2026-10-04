# ElevenLabs speech synthesis

Gateway route: `POST /v1/audio/speech` with `model`, `input`, and an optional
`voice` containing the ElevenLabs voice ID. The default voice/model are retained.
Responses are MP3 audio; the WebSocket bridge buffers one complete generation
before returning. This adapter does not expose a persistent realtime session,
multi-speaker dialogue or other ElevenLabs modalities.

| Models | Upstream protocol |
| --- | --- |
| `eleven_v3`, `eleven_multilingual_v2`, `eleven_flash_v2_5`, `eleven_flash_v2`, `eleven_turbo_v2_5`, `eleven_turbo_v2` | `/v1/text-to-speech/{voice_id}` |
| `eleven_v4` | `/v1/text-to-dialogue`, one `inputs` entry |
| `eleven_v3_conversational`, `eleven_v4_turbo` | `/v1/text-to-dialogue/stream-input` WebSocket, one registered voice |

Turbo v2/v2.5 are retained for compatibility although the upstream documentation
recommends Flash. Removed monolingual/multilingual v1 models are rejected before
requesting the provider.

The [model overview](https://elevenlabs.io/docs/overview/models) links v4 Turbo to
the Dialogue WebSocket, but the [WebSocket reference](https://elevenlabs.io/docs/api-reference/text-to-dialogue/ttd-websocket)
still says v3-only. The v4 Turbo mapping requires authenticated upstream testing.
Local protocol fixtures do not prove account entitlement or successful audio
generation with these newer models.

New models have per-input-character prices in `setting/ratio_setting/model_ratio.go`,
based on regular [API list prices](https://elevenlabs.io/pricing/api) on 2026-10-04.
Temporary promotions are excluded; existing model prices are preserved.
Existing database pricing options and channel model lists must be updated by
the operator; changing the built-in defaults does not rewrite stored options.

Complete channel model list:

```text
eleven_v4,eleven_v4_turbo,eleven_v3,eleven_v3_conversational,eleven_multilingual_v2,eleven_turbo_v2_5,eleven_flash_v2_5,eleven_flash_v2,eleven_turbo_v2
```

Checks: `go test ./relay/channel/elevenlabs -race`, plus controller
`TestElevenLabsCatalogConnectionTestRequests`. These exercise request conversion,
HTTP/WebSocket routing, API-key authentication, binary response conversion,
upstream failures, truncated audio, cancellation, and nonzero pricing coverage.
