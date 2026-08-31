package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userMsg returns a dto.Message with role "user" and string content.
func userMsg(content string) dto.Message {
	m := dto.Message{Role: "user"}
	m.SetStringContent(content)
	return m
}

// TestApplySystemPromptIfNeeded_InjectsForNonSkillRelay verifies that channel
// SystemPrompt is still injected for a normal request (regression coverage
// preserved from the deleted skill-relay test file during Skill Marketplace
// V1 removal; this behavior is not skill-specific).
func TestApplySystemPromptIfNeeded_InjectsForNonSkillRelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{}
	info.ChannelSetting.SystemPrompt = "Be concise."

	req := &dto.GeneralOpenAIRequest{Messages: []dto.Message{userMsg("hello")}}

	applySystemPromptIfNeeded(c, info, req)

	require.Len(t, req.Messages, 2,
		"channel SystemPrompt must be prepended")
	assert.Equal(t, "system", req.Messages[0].Role)
	assert.Equal(t, "Be concise.", req.Messages[0].StringContent())
	assert.Equal(t, "hello", req.Messages[1].StringContent())
}
