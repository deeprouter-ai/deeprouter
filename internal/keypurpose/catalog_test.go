package keypurpose

import (
	"github.com/QuantumNous/new-api/constant"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestMediaCatalogIncludesNewModelsWithoutStaticWhitelist(t *testing.T) {
	candidates := []Candidate{
		{"new-video-2027", []constant.EndpointType{VideoGeneration}},
		{"sora-test", []constant.EndpointType{constant.EndpointTypeOpenAIVideo}},
		{"new-image-2027", []constant.EndpointType{constant.EndpointTypeImageGeneration}},
		{"eleven_v4", MediaEndpoints(constant.ChannelTypeElevenLabs, "eleven_v4", nil)},
		{"chat-only", []constant.EndpointType{constant.EndpointTypeOpenAI}},
		{"new-video-2027", []constant.EndpointType{VideoGeneration}},
	}
	for purpose, expected := range map[string][]string{
		"video": {"new-video-2027", "sora-test"}, "image": {"new-image-2027"}, "voice": {"eleven_v4"},
	} {
		if actual := Models(purpose, candidates); !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s: got %v, want %v", purpose, actual, expected)
		}
	}
	if len(Models("video", nil)) != 0 {
		t.Fatal("empty catalog granted models")
	}
}

func TestMediaEndpointsDoNotTurnChatIntoVideo(t *testing.T) {
	chat := []constant.EndpointType{constant.EndpointTypeOpenAI}
	if actual := MediaEndpoints(constant.ChannelTypeMiniMax, "MiniMax-M2", chat); !reflect.DeepEqual(actual, chat) {
		t.Fatal(actual)
	}
	if actual := MediaEndpoints(constant.ChannelTypeMiniMax, "MiniMax-Hailuo-new", chat); !reflect.DeepEqual(actual, []constant.EndpointType{VideoGeneration}) {
		t.Fatal(actual)
	}
	if len(MediaEndpoints(constant.ChannelTypeElevenLabs, "eleven_multilingual_v1", chat)) != 0 {
		t.Fatal("retired voice model advertised")
	}
}

func TestMediaEndpointsRejectUnsupportedElevenLabsCapabilities(t *testing.T) {
	for _, name := range []string{"music_v2_5", "scribe_v2", "eleven_text_to_sound_v2", "eleven_ttv_v3", "unknown-eleven-model"} {
		if got := MediaEndpoints(constant.ChannelTypeElevenLabs, name, nil); len(got) != 0 {
			t.Errorf("unsupported capability %s advertised as %v", name, got)
		}
	}
}

// Adding a TTS adapter model must also make it discoverable with the speech
// protocol. This checks the source ModelList so a future addition cannot drift.
func TestMediaEndpointsCoverElevenLabsAdapterModels(t *testing.T) {
	content, err := os.ReadFile("../../relay/channel/elevenlabs/constant.go")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(content), "var ModelList = []string{")
	if len(parts) != 2 {
		t.Fatal("ElevenLabs ModelList declaration changed")
	}
	block := strings.Split(parts[1], "}")[0]
	matches := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatal("no adapter models parsed")
	}
	for _, match := range matches {
		got := MediaEndpoints(constant.ChannelTypeElevenLabs, match[1], nil)
		if !reflect.DeepEqual(got, []constant.EndpointType{constant.EndpointTypeAudioSpeech}) {
			t.Errorf("adapter model %s has no TTS discovery contract: %v", match[1], got)
		}
	}
}

func TestMediaEndpointsLabelSeedreamAsImageGeneration(t *testing.T) {
	chat := []constant.EndpointType{constant.EndpointTypeOpenAI}
	image := []constant.EndpointType{constant.EndpointTypeImageGeneration}
	for _, name := range []string{"doubao-seedream-4-0-250828", "doubao-seedream-4-5-251128", "doubao-seedream-5-0-260128", "doubao-seedream-5-0-pro-260628"} {
		if actual := MediaEndpoints(constant.ChannelTypeVolcEngine, name, chat); !reflect.DeepEqual(actual, image) {
			t.Fatalf("%s: got %v, want %v", name, actual, image)
		}
	}
	// Volcengine chat models keep their configured label.
	if actual := MediaEndpoints(constant.ChannelTypeVolcEngine, "doubao-seed-2-1-pro-260628", chat); !reflect.DeepEqual(actual, chat) {
		t.Fatalf("chat model relabelled: %v", actual)
	}
	// The image key purpose now picks Seedream up from the catalog.
	got := Models("image", []Candidate{{"doubao-seedream-5-0-260128", MediaEndpoints(constant.ChannelTypeVolcEngine, "doubao-seedream-5-0-260128", chat)}})
	if !reflect.DeepEqual(got, []string{"doubao-seedream-5-0-260128"}) {
		t.Fatalf("image purpose missed Seedream: %v", got)
	}
}
