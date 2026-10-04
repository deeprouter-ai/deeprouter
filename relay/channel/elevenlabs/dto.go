package elevenlabs

// ttsRequest is the body for POST {base}/v1/text-to-speech/{voice_id}.
type ttsRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id,omitempty"`
	VoiceSettings *voiceSettings `json:"voice_settings,omitempty"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}

// Single-voice dialogue preserves the gateway's OpenAI audio/speech contract.
type dialogueInput struct {
	Text    string `json:"text"`
	VoiceID string `json:"voice_id"`
}

type dialogueRequest struct {
	Inputs  []dialogueInput `json:"inputs"`
	ModelID string          `json:"model_id"`
}
