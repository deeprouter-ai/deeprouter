// Package keypurpose derives media key permissions from the enabled catalog.
package keypurpose

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const VideoGeneration constant.EndpointType = "video-generation"
const AudioTranscription constant.EndpointType = "audio-transcription"

func IsMedia(purpose string) bool {
	return purpose == "video" || purpose == "image" || purpose == "voice"
}

// MediaEndpoints corrects protocol labels for channels whose requests never
// use chat completions. Unknown channels retain their configured capabilities.
func MediaEndpoints(channelType int, name string, endpoints []constant.EndpointType) []constant.EndpointType {
	switch channelType {
	case constant.ChannelTypeElevenLabs:
		// This adapter only supports TTS. An upstream Music/Scribe/voice-design
		// model must not become a speech grant just because it shares a provider.
		switch name {
		case "eleven_v4", "eleven_v4_turbo", "eleven_v3", "eleven_v3_conversational",
			"eleven_multilingual_v2", "eleven_flash_v2_5", "eleven_flash_v2",
			"eleven_turbo_v2_5", "eleven_turbo_v2":
			return []constant.EndpointType{constant.EndpointTypeAudioSpeech}
		default:
			return nil
		}
	case constant.ChannelTypeKling, constant.ChannelTypeJimeng, constant.ChannelTypeVidu, constant.ChannelTypeDoubaoVideo:
		return []constant.EndpointType{VideoGeneration}
	case constant.ChannelTypeVolcEngine:
		// Seedream is Volcengine's image family; the adapter sends it to
		// /api/v3/images/generations. Without this it inherits the channel's
		// chat label, so the catalog tells AI tools to call chat completions
		// and image keys never receive it. Doubao chat models are untouched.
		if strings.HasPrefix(strings.ToLower(name), "doubao-seedream") {
			return []constant.EndpointType{constant.EndpointTypeImageGeneration}
		}
	case constant.ChannelTypeMiniMax:
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "minimax-h") || strings.HasPrefix(lower, "t2v-") || strings.HasPrefix(lower, "i2v-") || strings.HasPrefix(lower, "s2v-") {
			return []constant.EndpointType{VideoGeneration}
		}
		if strings.HasPrefix(name, "speech-") {
			return []constant.EndpointType{constant.EndpointTypeAudioSpeech}
		}
	}
	if strings.HasPrefix(name, "tts-") || strings.HasPrefix(name, "gpt-4o-mini-tts") {
		return []constant.EndpointType{constant.EndpointTypeAudioSpeech}
	}
	if strings.HasPrefix(name, "whisper-") || strings.HasPrefix(name, "gpt-4o-transcribe") || strings.HasPrefix(name, "gpt-4o-mini-transcribe") {
		return []constant.EndpointType{AudioTranscription}
	}
	return endpoints
}

type Candidate struct {
	Name      string
	Endpoints []constant.EndpointType
}

// Models returns an exact, deterministic snapshot, never wildcard grants or
// chat aliases. The caller must supply only enabled, billable group candidates.
func Models(purpose string, candidates []Candidate) []string {
	set := map[string]bool{}
	for _, candidate := range candidates {
		for _, endpoint := range candidate.Endpoints {
			matches := purpose == "video" && (endpoint == VideoGeneration || endpoint == constant.EndpointTypeOpenAIVideo) ||
				purpose == "image" && endpoint == constant.EndpointTypeImageGeneration ||
				purpose == "voice" && (endpoint == constant.EndpointTypeAudioSpeech || endpoint == AudioTranscription)
			if matches && candidate.Name != "" {
				set[candidate.Name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
