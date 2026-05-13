package relay

// Airbotix policy hooks: applied to the typed *dto.GeneralOpenAIRequest BEFORE
// it is converted to a provider-specific payload, so that downstream Claude /
// DeepSeek / etc. adapters translate the kid-safe constraints automatically.
//
// Behaviours (all gated by policy.Decision from middleware/policy.go):
//   - EnforceModelWhitelist: reject early with a 400 if the requested model is
//     not on kids.EligibleModels.
//   - InjectChildSafePrompt: prepend (or replace, when KidsMode=true) the
//     child-safe system prompt at messages[0].
//   - StripIdentifying: clear User + SafetyIdentifier on the request.
//   - EnforceZDR + OpenAI-family channel: force Store=false.
//
// See internal/kids and internal/policy for the V0 semantics this wires up.

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/kids"
	"github.com/QuantumNous/new-api/internal/policy"
)

// prependKidsSystemPrompt mutates request.Messages so that the child-safe
// system prompt is the effective system message seen by upstream.
//
// kidsMode=true is a hard constraint: any incoming system message is replaced.
// kidsMode=false (i.e. kid-safe profile alone) is softer: prepend only if no
// system message exists.
func prependKidsSystemPrompt(request *dto.GeneralOpenAIRequest, kidsMode bool) {
	if request == nil {
		return
	}
	role := request.GetSystemRoleName()
	prompt := kids.ChildSafeSystemPrompt()
	for i, m := range request.Messages {
		if m.Role == role || m.Role == "system" || m.Role == "developer" {
			if kidsMode {
				request.Messages[i].Role = role
				request.Messages[i].SetStringContent(prompt)
			}
			return
		}
	}
	sysMsg := dto.Message{Role: role}
	sysMsg.SetStringContent(prompt)
	request.Messages = append([]dto.Message{sysMsg}, request.Messages...)
}

// isOpenAIFamilyChannel returns true for channels where OpenAI's `store: false`
// Zero-Data-Retention flag is honoured. Non-OpenAI providers ignore the field
// (or worse, reject it), so we limit ZDR injection to the OpenAI family.
func isOpenAIFamilyChannel(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeOpenAIMax:
		return true
	}
	return false
}

// applyAirbotixPolicy is the single mutation entry-point called from TextHelper.
// Returns a non-nil reject string when the model whitelist check denies the
// request; otherwise mutates request in place and returns "".
func applyAirbotixPolicy(decision policy.Decision, channelType int, request *dto.GeneralOpenAIRequest) (rejectReason string) {
	if request == nil {
		return ""
	}
	if decision.EnforceModelWhitelist && !kids.IsModelEligible(request.Model) {
		return "model_not_eligible_for_kids_mode: " + request.Model
	}
	if decision.InjectChildSafePrompt {
		prependKidsSystemPrompt(request, decision.KidsMode)
	}
	if decision.StripIdentifying {
		request.User = nil
		request.SafetyIdentifier = nil
	}
	if decision.EnforceZDR && isOpenAIFamilyChannel(channelType) {
		request.Store = json.RawMessage("false")
	}
	return ""
}
