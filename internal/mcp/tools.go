package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/internal/discovery"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// autoModel is DeepRouter's smart-routing alias for chat.
const autoModel = "deeprouter-auto"

// register adds the five tools. Every one only reads.
func (d *desk) register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_models",
		Description: "List the models the user's DeepRouter key can use right now, each with what it is for (chat, image, video, speech, transcription, embedding, rerank), the endpoint to call and its price. Without a key, lists the public catalog instead (scope=public). Call this before choosing a model - never guess a model id.",
		Annotations: readOnly("List models"),
	}, d.listModels)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_model",
		Description: "How to call one model: for each endpoint it supports, the full URL, auth header, required parameters, a minimal request body and what comes back - for video, the submit, poll and download steps. Also its price, and whether the user's key can use it.",
		Annotations: readOnly("Get model details"),
	}, d.getModel)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_connection_config",
		Description: "The exact settings for pointing a client app or SDK at DeepRouter - protocol, base URL, auth header, a model to start with and the known pitfalls - taken from DeepRouter's published guide for that client. Pass the client (claude-code, codex, cursor, cherry-studio, openai-sdk..., or others for a tool not listed); pass mcp for adding this server to Claude Code, Codex or Cursor; leave it out to get the list.",
		Annotations: readOnly("Get connection settings"),
	}, d.connectionConfig)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_key",
		Description: "Check the user's DeepRouter key: whether it works and if not why, its own spending limit, expiry, group, model whitelist and the balance that pays for it. Never shows the key itself.",
		Annotations: readOnly("Check API key"),
	}, d.checkKey)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_started",
		Description: "What DeepRouter is, the base URL for each protocol, how to sign up and get a key, and which of these tools answers what.",
		Annotations: readOnly("Get started"),
	}, d.getStarted)
}

// readOnly marks a tool that only reads and reaches nothing outside DeepRouter.
func readOnly(title string) *mcp.ToolAnnotations {
	closedWorld := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closedWorld}
}

// ---- list_models ----

// listModelsInput is what list_models may be asked.
type listModelsInput struct {
	Purpose string `json:"purpose,omitempty" jsonschema:"only models for this purpose: chat, image, video, speech, transcription, embedding, rerank, audio (speech and transcription) or all (the default)"`
}

// modelEntry is one model in list_models' answer.
type modelEntry struct {
	ID       string   `json:"id"`
	Purposes []string `json:"purposes"`
	Endpoint string   `json:"endpoint" jsonschema:"method and URL of the endpoint the gateway prefers for this model; get_model has the others"`
	Types    []string `json:"supported_endpoint_types"`
	Price    string   `json:"price"`
	Async    bool     `json:"async,omitempty" jsonschema:"true for video: submit a task, then poll for the result (see get_model)"`
	Note     string   `json:"note,omitempty"`
}

// listModelsOutput is list_models' answer.
type listModelsOutput struct {
	Scope     string       `json:"scope" jsonschema:"key: what this key can use; public: the public catalog because no key was sent; none: the key cannot be used, see message"`
	KeyStatus string       `json:"key_status"`
	Message   string       `json:"message"`
	Count     int          `json:"count"`
	Models    []modelEntry `json:"models"`
}

// listModels answers list_models: the key's models, or the public catalog without a key.
func (d *desk) listModels(ctx context.Context, _ *mcp.CallToolRequest, in listModelsInput) (*mcp.CallToolResult, listModelsOutput, error) {
	only, err := purposeFilter(in.Purpose)
	if err != nil {
		return nil, listModelsOutput{}, err
	}
	c := callerFrom(ctx)
	state := c.key()
	out := listModelsOutput{KeyStatus: state.Status, Models: []modelEntry{}}

	type listed struct {
		id    string
		types []constant.EndpointType
	}
	var keys map[string]any
	switch state.Status {
	case statusOK:
		keys = state.auth.Keys
		out.Scope = "key"
		out.Message = "The models this key can use right now."
	case statusNoKey:
		out.Scope = "public"
		out.Message = "No key was sent, so this is DeepRouter's public catalog, not what any one key can use. " + keyProblem(state)
	default:
		out.Scope = "none"
		out.Message = keyProblem(state)
		return nil, out, nil
	}

	// The catalog comes first: building it is also what loads each model's
	// endpoint types, which the /v1/models handler reads.
	prices, err := c.pricing(keys)
	if err != nil {
		return nil, out, err
	}
	var models []listed
	if state.Status == statusOK {
		keyModels, err := c.keyModels(state)
		if err != nil {
			return nil, out, fmt.Errorf("could not list this key's models: %w", err)
		}
		for _, m := range keyModels {
			models = append(models, listed{m.Id, m.SupportedEndpointTypes})
		}
	} else {
		for id, p := range prices.byName {
			models = append(models, listed{id, p.SupportedEndpointTypes})
		}
	}
	for _, m := range models {
		purposes := purposesOf(m.types)
		if only != nil && !anyOf(purposes, only) {
			continue
		}
		entry := modelEntry{
			ID:       m.id,
			Purposes: purposes,
			Endpoint: purposeUnknown,
			Types:    typeNames(m.types),
			Price:    prices.priceOf(m.id, purposes).Line,
			Async:    isAsync(m.types),
		}
		if endpoints := endpointsOf(m.id, m.types); len(endpoints) > 0 && endpoints[0].URL != purposeUnknown {
			entry.Endpoint = endpoints[0].Method + " " + endpoints[0].URL
		}
		if m.id == autoModel {
			entry.Note = autoModelNote
		}
		out.Models = append(out.Models, entry)
	}
	sort.Slice(out.Models, func(i, j int) bool {
		a, b := rank(out.Models[i].Purposes[0]), rank(out.Models[j].Purposes[0])
		if a != b {
			return a < b
		}
		return out.Models[i].ID < out.Models[j].ID
	})
	out.Count = len(out.Models)
	return nil, out, nil
}

// purposeFilter reads list_models' purpose argument; nil means every purpose.
func purposeFilter(purpose string) (map[string]bool, error) {
	switch p := strings.ToLower(strings.TrimSpace(purpose)); p {
	case "", "all":
		return nil, nil
	case "audio":
		return map[string]bool{purposeSpeech: true, purposeTranscription: true}, nil
	case purposeChat, purposeImage, purposeVideo, purposeSpeech, purposeTranscription, purposeEmbedding, purposeRerank:
		return map[string]bool{p: true}, nil
	}
	return nil, fmt.Errorf("unknown purpose %q: use chat, image, video, speech, transcription, embedding, rerank, audio or all", purpose)
}

// anyOf reports whether a model serves any of the wanted purposes.
func anyOf(purposes []string, wanted map[string]bool) bool {
	for _, p := range purposes {
		if wanted[p] {
			return true
		}
	}
	return false
}

// rank is a purpose's place in the listing order.
func rank(purpose string) int {
	for i, p := range purposeOrder {
		if p == purpose {
			return i
		}
	}
	return len(purposeOrder)
}

// typeNames writes endpoint types as the strings /v1/models uses.
func typeNames(types []constant.EndpointType) []string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, string(t))
	}
	return names
}

// ---- get_model ----

// getModelInput is what get_model is asked.
type getModelInput struct {
	ID string `json:"id" jsonschema:"the exact model id, as list_models returns it"`
}

// getModelOutput is get_model's answer.
type getModelOutput struct {
	ID        string           `json:"id"`
	Scope     string           `json:"scope" jsonschema:"key: checked against this key; public: from the public catalog"`
	KeyStatus string           `json:"key_status"`
	Available string           `json:"available_to_key" jsonschema:"yes, no, or unknown when no usable key was sent"`
	Message   string           `json:"message"`
	Purposes  []string         `json:"purposes"`
	Endpoints []endpointDetail `json:"endpoints"`
	Pricing   priceDetail      `json:"pricing"`
	DocsURL   string           `json:"docs_url" jsonschema:"the provider's documentation for this model, or unknown"`
	ModelPage string           `json:"model_page"`
	Note      string           `json:"note,omitempty"`
}

// getModel answers get_model: how to call one model, what it costs, and whether the key may.
func (d *desk) getModel(ctx context.Context, _ *mcp.CallToolRequest, in getModelInput) (*mcp.CallToolResult, getModelOutput, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, getModelOutput{}, errors.New("id is required: pass a model id from list_models")
	}
	c := callerFrom(ctx)
	state := c.key()
	out := getModelOutput{
		ID:        id,
		KeyStatus: state.Status,
		// The provider documentation links live only in the console's import
		// presets, not on the server (PRD Q8): unknown, never a guess.
		DocsURL:   purposeUnknown,
		ModelPage: pricingURL + "/" + url.PathEscape(id),
	}

	var keys map[string]any
	if state.Status == statusOK {
		keys = state.auth.Keys
	}
	// The catalog first, as in list_models: it loads the endpoint types.
	prices, err := c.pricing(keys)
	if err != nil {
		return nil, getModelOutput{}, err
	}

	var types []constant.EndpointType
	listed := false
	if state.Status == statusOK {
		keyModels, err := c.keyModels(state)
		if err != nil {
			return nil, getModelOutput{}, fmt.Errorf("could not list this key's models: %w", err)
		}
		for _, m := range keyModels {
			if m.Id == id {
				types, listed = m.SupportedEndpointTypes, true
				break
			}
		}
		out.Scope = "key"
		if listed {
			out.Available = "yes"
			out.Message = "This key can use this model."
		} else {
			out.Available = "no"
			out.Message = "This key cannot use this model: it is outside the key's model whitelist or group. Pick one from list_models, or change the key at " + keysURL + "."
		}
	} else {
		out.Scope = "public"
		out.Available = purposeUnknown
		out.Message = keyProblem(state)
	}

	if !listed {
		p, ok := prices.byName[id]
		if !ok {
			return nil, getModelOutput{}, fmt.Errorf("DeepRouter has no model %q: call list_models for the exact ids", id)
		}
		types = p.SupportedEndpointTypes
	}
	out.Purposes = purposesOf(types)
	out.Endpoints = endpointsOf(id, types)
	out.Pricing = prices.priceOf(id, out.Purposes)
	if id == autoModel {
		out.Note = autoModelNote
	}
	return nil, out, nil
}

// ---- get_connection_config ----

// connectionConfigInput is what get_connection_config may be asked.
type connectionConfigInput struct {
	Client string `json:"client,omitempty" jsonschema:"the client, named as in its guide: claude-code, codex, cursor, cherry-studio, openai-sdk... or others for a tool that has no guide, or mcp for adding this server to a client; leave it out to list the clients"`
}

// connectionConfigOutput is get_connection_config's answer.
type connectionConfigOutput struct {
	Client  string         `json:"client,omitempty"`
	Facts   string         `json:"facts_yaml,omitempty" jsonschema:"the guide's Facts for AI assistants block, verbatim"`
	Fields  map[string]any `json:"facts,omitempty" jsonschema:"the same block, parsed"`
	Guide   string         `json:"guide,omitempty" jsonschema:"the whole guide, in Markdown"`
	Clients []string       `json:"clients,omitempty" jsonschema:"every client with a guide, when none was asked for or the one asked for has none"`
	Message string         `json:"message"`
}

// connectionConfig answers get_connection_config from the published guide for the client.
func (d *desk) connectionConfig(_ context.Context, _ *mcp.CallToolRequest, in connectionConfigInput) (*mcp.CallToolResult, connectionConfigOutput, error) {
	var out connectionConfigOutput
	if len(d.guides.slugs) == 0 {
		out.Message = "The integration guides are not available on this server; read " + discovery.GuideURL + " instead."
		return nil, out, nil
	}
	clients := append([]string{}, d.guides.slugs...)
	if strings.TrimSpace(in.Client) == "" {
		out.Clients = clients
		out.Message = "Call again with one of clients. For a tool that is not listed, use others."
		return nil, out, nil
	}
	g, ok := d.guides.find(in.Client)
	if !ok {
		out.Clients = clients
		out.Message = fmt.Sprintf("There is no guide for %q. Use one of clients; for a tool that is not listed, use others.", in.Client)
		return nil, out, nil
	}
	out.Client = g.slug
	out.Facts = g.facts
	out.Fields = g.fields
	out.Guide = guidesURL + g.slug + ".md"
	out.Message = "Copy these values exactly; facts_yaml is the guide's own block."
	return nil, out, nil
}

// ---- check_key ----

// checkKeyOutput is check_key's answer.
type checkKeyOutput struct {
	Status          string   `json:"status" jsonschema:"ok, no_key, invalid, disabled, expired, exhausted, refused or error"`
	Message         string   `json:"message"`
	KeyName         string   `json:"key_name,omitempty"`
	Group           string   `json:"group,omitempty"`
	ExpiresAt       string   `json:"expires_at,omitempty" jsonschema:"never, or when the key stops working (UTC)"`
	KeyLimit        string   `json:"key_limit,omitempty" jsonschema:"unlimited, or what is left of the key's own spending limit"`
	KeyRemainingUSD *float64 `json:"key_remaining_usd,omitempty"`
	PaidBy          string   `json:"paid_by,omitempty" jsonschema:"personal account or organization"`
	BalanceUSD      *float64 `json:"balance_usd,omitempty" jsonschema:"the personal account's balance; an organization's balance is not shown to key holders"`
	ModelWhitelist  []string `json:"model_whitelist,omitempty" jsonschema:"the only models this key may call; absent means every model its group offers"`
	GetAKey         string   `json:"get_a_key"`
}

// checkKey answers check_key: whether the key works, and what it may spend.
func (d *desk) checkKey(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, checkKeyOutput, error) {
	state := callerFrom(ctx).key()
	out := checkKeyOutput{Status: state.Status, GetAKey: keysURL}
	if t := state.Token; t != nil {
		out.KeyName = t.Name
		out.ExpiresAt = expiry(t.ExpiredTime)
		if t.UnlimitedQuota {
			out.KeyLimit = "unlimited"
		} else {
			left := usdOf(t.RemainQuota)
			out.KeyRemainingUSD = &left
			out.KeyLimit = usd(left) + " left of this key's own limit"
		}
		if t.ModelLimitsEnabled {
			for name := range t.GetModelLimitsMap() {
				out.ModelWhitelist = append(out.ModelWhitelist, name)
			}
			sort.Strings(out.ModelWhitelist)
		}
	}
	if state.Status != statusOK {
		out.Message = keyProblem(state)
		return nil, out, nil
	}

	keys := state.auth.Keys
	out.Group, _ = keys[string(constant.ContextKeyUsingGroup)].(string)
	if state.Token.OrgId != 0 {
		out.PaidBy = "organization"
		out.Message = "This key works. Its organization's wallet pays for it; that balance is shown only to the organization's owner and admins, so if requests are refused for lack of quota, ask one of them."
		return nil, out, nil
	}
	out.PaidBy = "personal account"
	quota, _ := keys[string(constant.ContextKeyUserQuota)].(int)
	balance := usdOf(quota)
	out.BalanceUSD = &balance
	out.Message = "This key works."
	if quota <= 0 {
		out.Message += " The account balance is $0, so requests will be refused unless a subscription plan covers them; top up at " + walletURL + "."
	}
	return nil, out, nil
}

// expiry says when a key stops working.
func expiry(expiredTime int64) string {
	if expiredTime == -1 {
		return "never"
	}
	return time.Unix(expiredTime, 0).UTC().Format(time.RFC3339)
}

// usdOf converts the gateway's quota units to US dollars of credit.
func usdOf(quota int) float64 {
	return round(float64(quota) / common.QuotaPerUnit)
}

// keyProblem says, for a person, what is wrong with the key and what to do.
func keyProblem(state *keyState) string {
	switch state.Status {
	case statusNoKey:
		if state.Message != "" {
			return "No key arrived: the Authorization header holds the text " + state.Message + " itself, so that environment variable is not set where the client runs. Run DeepRouter's one-click setup (console -> API Keys), which saves the key in that variable, then open a new terminal; or add this server again with the key written in place of the variable. Get a key at " + keysURL + "."
		}
		return "Send the user's DeepRouter key with every request to this server, as the header \"Authorization: Bearer <key>\". Get a key at " + keysURL + "."
	case statusInvalid:
		return "DeepRouter has no such key. Check that it was copied in full, or create one at " + keysURL + "."
	case statusDisabled:
		if state.Message != "" {
			return "This key cannot be used: " + state.Message + "."
		}
		return "This key is disabled. Turn it back on, or use another key, at " + keysURL + "."
	case statusExpired:
		return "This key has expired. Extend it, or create a new one, at " + keysURL + "."
	case statusExhausted:
		return "This key has used up its own spending limit. Raise the limit, or use another key, at " + keysURL + "."
	case statusRefused:
		return "DeepRouter refused this key: " + state.Message
	case statusError:
		return "DeepRouter could not check this key just now. Try again in a moment."
	}
	return ""
}

// ---- get_started ----

// getStartedOutput is get_started's answer.
type getStartedOutput struct {
	About     string            `json:"about"`
	BaseURLs  map[string]string `json:"base_urls" jsonschema:"by the protocol a tool speaks"`
	Auth      string            `json:"auth"`
	FirstCall string            `json:"first_call"`
	SignUp    string            `json:"sign_up"`
	GetAKey   string            `json:"get_a_key"`
	Reference string            `json:"api_reference"`
	Tools     map[string]string `json:"tools"`
	Note      string            `json:"note"`
}

// getStarted answers get_started with the same introduction for everyone.
func (d *desk) getStarted(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, getStartedOutput, error) {
	return nil, getStartedOutput{
		About: "DeepRouter is one API key for many AI models - chat, image, video, speech and more - behind OpenAI-, Anthropic- and Gemini-compatible endpoints. Point a tool at it by changing two things: the base URL and the key.",
		BaseURLs: map[string]string{
			"openai":    apiBase + "/v1",
			"anthropic": apiBase,
			"gemini":    apiBase + "/v1beta",
		},
		Auth:      "Authorization: Bearer <your DeepRouter key> (Anthropic tools may send x-api-key, Gemini tools x-goog-api-key)",
		FirstCall: "GET " + apiBase + "/v1/models with the key lists what it can use; list_models does the same with prices.",
		SignUp:    signUpURL,
		GetAKey:   keysURL,
		Reference: discovery.GuideURL,
		Tools: map[string]string{
			"list_models":           "which models the key can use, with endpoint and price",
			"get_model":             "how to call one model, and its price",
			"get_connection_config": "the exact settings for one client app or SDK",
			"check_key":             "whether the key works, its limit, expiry and balance",
		},
		Note: "Some tools add the version segment themselves and take the host only (Cherry Studio, NextChat, Gemini CLI); get_connection_config has the exact value for each. This server only answers questions - generating anything means calling the API with the key.",
	}, nil
}
