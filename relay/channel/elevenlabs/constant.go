package elevenlabs

// ElevenLabs is a text-to-speech (voice) provider. It is NOT OpenAI-compatible:
// auth is the `xi-api-key` header. Standard TTS uses a voice ID in the URL;
// v4 uses single-voice Dialogue HTTP, and conversational models use the Dialogue
// WebSocket bridged to an HTTP audio response. Only TTS is supported here.

const (
	ChannelName = "elevenlabs"
	// Default "Rachel" voice — used when the request omits `voice`.
	defaultVoiceID = "21m00Tcm4TlvDq8ikWAM"
	defaultModelID = "eleven_multilingual_v2"
)

// ModelList is the set of ElevenLabs TTS model ids surfaced to the gateway.
// These map to the model_id field of the ElevenLabs TTS request and to the
// price keys in setting/ratio_setting/model_ratio.go.
var ModelList = []string{
	"eleven_v4",
	"eleven_v4_turbo",
	"eleven_v3",
	"eleven_v3_conversational",
	"eleven_multilingual_v2",
	"eleven_turbo_v2_5",
	"eleven_flash_v2_5",
	"eleven_flash_v2",
	"eleven_turbo_v2",
}

func usesDialogueHTTP(modelID string) bool { return modelID == "eleven_v4" }

func usesDialogueWebSocket(modelID string) bool {
	return modelID == "eleven_v3_conversational" || modelID == "eleven_v4_turbo"
}
