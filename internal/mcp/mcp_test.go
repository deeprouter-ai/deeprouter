package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// gateway is a real gin engine over a throwaway database, serving /mcp next
// to /v1/models: keys are checked, models listed and prices read by the
// gateway's own code, and only the network is local.
type gateway struct {
	url  string
	keys map[string]string // fixture name -> the key as its holder has it
}

// setGlobal sets a package variable for one test and puts it back after.
func setGlobal[T any](t *testing.T, target *T, value T) {
	t.Helper()
	previous := *target
	*target = value
	t.Cleanup(func() { *target = previous })
}

// newGateway builds the gateway with three channels - chat and image, speech,
// video - and keys covering every state check_key can report.
func newGateway(t *testing.T, db *gorm.DB) *gateway {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	require.NoError(t, orgmodel.Migrate(db))
	dialect := db.Dialector.Name()
	setGlobal(t, &model.DB, db)
	setGlobal(t, &model.LOG_DB, db)
	setGlobal(t, &common.UsingSQLite, dialect == "sqlite")
	setGlobal(t, &common.UsingPostgreSQL, dialect == "postgres")
	setGlobal(t, &common.UsingMySQL, dialect == "mysql")
	setGlobal(t, &common.RedisEnabled, false)
	setGlobal(t, &common.MemoryCacheEnabled, false)
	model.InitColumnNames()
	ratio_setting.InitRatioSettings()
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)

	base := "http://127.0.0.1:9"
	for _, ch := range []model.Channel{
		{Type: constant.ChannelTypeOpenAI, Name: "openai", Key: "sk-upstream-1", Models: "gpt-4o-mini,gpt-image-1,tts-1"},
		{Type: constant.ChannelTypeMiniMax, Name: "minimax", Key: "sk-upstream-2", Models: "MiniMax-H3"},
	} {
		ch.Status = common.ChannelStatusEnabled
		ch.Group = "default"
		ch.BaseURL = &base
		require.NoError(t, ch.Insert())
	}

	user := func(name string, quota int, status int, orgID int) *model.User {
		u := &model.User{Username: name, Password: "unused-password", AffCode: name, Status: status, Quota: quota, Group: "default", OrgId: orgID}
		require.NoError(t, db.Create(u).Error)
		return u
	}
	alice := user("alice", 1_000_000, common.UserStatusEnabled, 0)
	bob := user("bob", 0, common.UserStatusEnabled, 0)
	eve := user("eve", 1_000_000, common.UserStatusDisabled, 0)
	carol := user("carol", 5_000_000, common.UserStatusEnabled, 0)
	org := &orgmodel.Organization{Name: "Acme", OwnerUserId: carol.Id, CreatedTime: time.Now().Unix()}
	require.NoError(t, db.Create(org).Error)
	dave := user("dave", 0, common.UserStatusEnabled, org.Id)

	gw := &gateway{keys: map[string]string{}}
	key := func(name string, owner *model.User, edit func(*model.Token)) {
		tk := &model.Token{
			UserId: owner.Id, Name: name, Key: "mcptest" + name + strings.Repeat("x", 30),
			Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true,
		}
		if edit != nil {
			edit(tk)
		}
		require.NoError(t, db.Create(tk).Error)
		gw.keys[name] = "sk-" + tk.Key
	}
	key("all", alice, nil)
	key("wild", alice, func(tk *model.Token) { tk.ModelLimitsEnabled, tk.ModelLimits = true, "gpt-*" })
	key("video", alice, func(tk *model.Token) { tk.ModelLimitsEnabled, tk.ModelLimits = true, "MiniMax-H3" })
	key("limited", alice, func(tk *model.Token) {
		tk.UnlimitedQuota, tk.RemainQuota, tk.ExpiredTime = false, 250_000, time.Now().Add(24*time.Hour).Unix()
	})
	key("expired", alice, func(tk *model.Token) { tk.ExpiredTime = time.Now().Add(-time.Hour).Unix() })
	key("disabled", alice, func(tk *model.Token) { tk.Status = common.TokenStatusDisabled })
	key("spent", alice, func(tk *model.Token) { tk.UnlimitedQuota, tk.RemainQuota = false, 0 })
	key("ip", alice, func(tk *model.Token) { ips := "10.9.8.7"; tk.AllowIps = &ips })
	key("broke", bob, nil)
	key("banned", eve, nil)
	key("org", dave, func(tk *model.Token) { tk.OrgId = org.Id })

	engine := gin.New()
	engine.Any(Path, NewHandler(engine, os.DirFS(publishedGuides)))
	// /v1/models as router/relay-router.go registers it for a Bearer key.
	engine.GET("/v1/models", middleware.TokenAuth(), func(c *gin.Context) {
		controller.ListModels(c, constant.ChannelTypeOpenAI)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	gw.url = server.URL
	return gw
}

// withKey adds a key to every request, as an MCP client configured with an
// Authorization header does.
type withKey struct {
	key string
}

// RoundTrip sends the request with the key attached.
func (w withKey) RoundTrip(r *http.Request) (*http.Response, error) {
	if w.key != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+w.key)
	}
	return http.DefaultTransport.RoundTrip(r)
}

// connect opens an MCP session the way a client does, with the SDK's own
// client over HTTP; key "" connects without one.
func (gw *gateway) connect(t *testing.T, key string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "deeprouter-mcp-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   gw.url + Path,
		HTTPClient: &http.Client{Transport: withKey{key}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// call runs a tool and decodes its structured answer. A protocol error fails
// the test: it would mean the client was cut off instead of told.
func call[T any](t *testing.T, s *mcp.ClientSession, tool string, args map[string]any) (T, *mcp.CallToolResult) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err, tool)
	var out T
	if !res.IsError {
		raw, err := common.Marshal(res.StructuredContent)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(raw, &out))
	}
	return out, res
}

// v1Models is what GET /v1/models answers for a key, as sorted ids.
func (gw *gateway) v1Models(t *testing.T, key string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, gw.url+"/v1/models", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(body, &answer))
	ids := make([]string, 0, len(answer.Data))
	for _, m := range answer.Data {
		ids = append(ids, m.Id)
	}
	sort.Strings(ids)
	return ids
}

// ids is the sorted model ids of a list_models answer.
func ids(models []modelEntry) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	sort.Strings(out)
	return out
}

// TestMCP_ListModelsIsWhatV1ModelsAnswers is acceptance A1: for keys with no
// whitelist, a wildcard whitelist and a one-model whitelist, list_models names
// exactly the models GET /v1/models names, each with purpose, endpoint and a
// price; and purpose=video leaves only video models.
func TestMCP_ListModelsIsWhatV1ModelsAnswers(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		want := map[string][]string{
			"all":   {"MiniMax-H3", "gpt-4o-mini", "gpt-image-1", "tts-1"},
			"wild":  {"gpt-4o-mini", "gpt-image-1"},
			"video": {"MiniMax-H3"},
		}
		for name, expected := range want {
			key := gw.keys[name]
			require.Equal(t, expected, gw.v1Models(t, key), "/v1/models for %s", name)

			out, res := call[listModelsOutput](t, gw.connect(t, key), "list_models", nil)
			require.False(t, res.IsError, name)
			require.Equal(t, "key", out.Scope)
			require.Equal(t, statusOK, out.KeyStatus)
			require.Equal(t, expected, ids(out.Models), "list_models for %s", name)
			require.Equal(t, len(expected), out.Count)
			for _, m := range out.Models {
				require.NotEmpty(t, m.Purposes, m.ID)
				require.True(t, strings.HasPrefix(m.Endpoint, "POST "+apiBase+"/v1/"), "%s: %s", m.ID, m.Endpoint)
				require.NotEmpty(t, m.Price, m.ID)
				require.NotContains(t, m.Price, "unknown", m.ID)
			}
		}

		videoOnly, _ := call[listModelsOutput](t, gw.connect(t, gw.keys["all"]), "list_models", map[string]any{"purpose": "video"})
		require.Equal(t, []string{"MiniMax-H3"}, ids(videoOnly.Models))
		require.True(t, videoOnly.Models[0].Async)

		_, res := call[listModelsOutput](t, gw.connect(t, gw.keys["all"]), "list_models", map[string]any{"purpose": "music"})
		require.True(t, res.IsError, "an unknown purpose is a tool error, not a dropped connection")
	})
}

// TestMCP_WithoutAKey is acceptance A2: a client with no key connects, sees the
// five tools marked read-only, and gets the public catalog, a model's details,
// a client's settings and the introduction; check_key says there is no key and
// where to get one instead of failing.
func TestMCP_WithoutAKey(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		s := gw.connect(t, "")

		tools, err := s.ListTools(context.Background(), nil)
		require.NoError(t, err)
		var names []string
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
			require.True(t, tool.Annotations.ReadOnlyHint, tool.Name)
		}
		sort.Strings(names)
		require.Equal(t, []string{"check_key", "get_connection_config", "get_model", "get_started", "list_models"}, names)

		list, res := call[listModelsOutput](t, s, "list_models", nil)
		require.False(t, res.IsError)
		require.Equal(t, "public", list.Scope)
		require.Equal(t, statusNoKey, list.KeyStatus)
		require.Equal(t, []string{"MiniMax-H3", "gpt-4o-mini", "gpt-image-1", "tts-1"}, ids(list.Models))

		m, res := call[getModelOutput](t, s, "get_model", map[string]any{"id": "gpt-4o-mini"})
		require.False(t, res.IsError)
		require.Equal(t, "public", m.Scope)
		require.Equal(t, purposeUnknown, m.Available)

		cfg, res := call[connectionConfigOutput](t, s, "get_connection_config", map[string]any{"client": "codex"})
		require.False(t, res.IsError)
		require.Equal(t, "codex", cfg.Client)
		require.Contains(t, cfg.Facts, "base_url:")

		started, res := call[getStartedOutput](t, s, "get_started", nil)
		require.False(t, res.IsError)
		require.Equal(t, apiBase+"/v1", started.BaseURLs["openai"])

		check, res := call[checkKeyOutput](t, s, "check_key", nil)
		require.False(t, res.IsError)
		require.Equal(t, statusNoKey, check.Status)
		require.Equal(t, keysURL, check.GetAKey)
		require.Contains(t, check.Message, "Authorization: Bearer")
	})
}

// TestMCP_AVariableNameIsNotAKey covers the keys page's command, whose header
// reads `${ANTHROPIC_AUTH_TOKEN}` for the client to expand: with no such
// variable where the client runs, the text itself arrives. That is no key,
// named back as the unset variable - not "no such key", which would send the
// user off to copy a key that was never the problem.
func TestMCP_AVariableNameIsNotAKey(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		s := gw.connect(t, "${ANTHROPIC_AUTH_TOKEN}")

		check, res := call[checkKeyOutput](t, s, "check_key", nil)
		require.False(t, res.IsError)
		require.Equal(t, statusNoKey, check.Status)
		require.Contains(t, check.Message, "${ANTHROPIC_AUTH_TOKEN}")
		require.Contains(t, check.Message, "one-click setup")
		require.NotContains(t, check.Message, "no such key")

		list, res := call[listModelsOutput](t, s, "list_models", nil)
		require.False(t, res.IsError)
		require.Equal(t, "public", list.Scope)
		require.Equal(t, statusNoKey, list.KeyStatus)
		require.Contains(t, list.Message, "${ANTHROPIC_AUTH_TOKEN}")
		require.NotEmpty(t, list.Models)
	})
}

// TestMCP_ABadKeyIsExplainedNotDropped is acceptance A3: an expired, a
// disabled and a used-up key - and a key refused for its client IP, one whose
// account is banned and one that does not exist - each get their own status
// and a sentence from check_key, list_models explains instead of listing,
// and the same session keeps working afterwards.
func TestMCP_ABadKeyIsExplainedNotDropped(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		cases := map[string]string{
			"expired":  statusExpired,
			"disabled": statusDisabled,
			"spent":    statusExhausted,
			"ip":       statusRefused,
			"banned":   statusDisabled,
		}
		gw.keys["unknown"] = "sk-mcptestnosuchkey"
		cases["unknown"] = statusInvalid
		for name, status := range cases {
			s := gw.connect(t, gw.keys[name])

			check, res := call[checkKeyOutput](t, s, "check_key", nil)
			require.False(t, res.IsError, name)
			require.Equal(t, status, check.Status, name)
			require.NotEmpty(t, check.Message, name)

			list, res := call[listModelsOutput](t, s, "list_models", nil)
			require.False(t, res.IsError, name)
			require.Equal(t, "none", list.Scope, name)
			require.Equal(t, status, list.KeyStatus, name)
			require.Equal(t, check.Message, list.Message, name)
			require.Empty(t, list.Models, name)

			started, res := call[getStartedOutput](t, s, "get_started", nil)
			require.False(t, res.IsError, name)
			require.NotEmpty(t, started.About, name)
		}

		ip, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["ip"]), "check_key", nil)
		require.Contains(t, ip.Message, "IP", "the gateway's own reason comes through")
		banned, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["banned"]), "check_key", nil)
		require.Contains(t, banned.Message, "account")
	})
}

// TestMCP_GetModelSaysHowToCallIt is acceptance A4: for a chat, an image, a
// video and a speech model, get_model gives the endpoint the gateway serves it
// on, the parameters, the async steps for video and the billing unit the
// gateway charges by - and "unknown" where the server has nothing to say.
func TestMCP_GetModelSaysHowToCallIt(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		s := gw.connect(t, gw.keys["all"])
		get := func(id string) getModelOutput {
			out, res := call[getModelOutput](t, s, "get_model", map[string]any{"id": id})
			require.False(t, res.IsError, id)
			require.Equal(t, "key", out.Scope, id)
			require.Equal(t, "yes", out.Available, id)
			require.Equal(t, purposeUnknown, out.DocsURL, id)
			require.Equal(t, pricingURL+"/"+id, out.ModelPage, id)
			return out
		}

		chat := get("gpt-4o-mini")
		require.Equal(t, []string{purposeChat}, chat.Purposes)
		require.Equal(t, "POST", chat.Endpoints[0].Method)
		require.Equal(t, apiBase+"/v1/chat/completions", chat.Endpoints[0].URL)
		require.Equal(t, "model, messages", chat.Endpoints[0].Params)
		// The price an AI quotes is what the key pays: the model's ratio, and the
		// markup of the key's group on top.
		ratio, _, _ := ratio_setting.GetModelRatio("gpt-4o-mini")
		group := ratio_setting.GetGroupRatio("default")
		require.Equal(t, "per_1m_tokens", chat.Pricing.Unit)
		require.Equal(t, group, chat.Pricing.GroupRatio)
		require.InDelta(t, ratio*2*group, *chat.Pricing.InputUSD, 1e-9)
		require.InDelta(t, ratio*2*group*ratio_setting.GetCompletionRatio("gpt-4o-mini"), *chat.Pricing.OutputUSD, 1e-9)

		image := get("gpt-image-1")
		require.Equal(t, purposeImage, image.Endpoints[0].Purpose)
		require.Equal(t, apiBase+"/v1/images/generations", image.Endpoints[0].URL)
		require.Equal(t, "model, prompt", image.Endpoints[0].Params)

		video := get("MiniMax-H3")
		require.Equal(t, []string{purposeVideo}, video.Purposes)
		require.Equal(t, apiBase+"/v1/video/generations", video.Endpoints[0].URL)
		require.Len(t, video.Endpoints[0].Steps, 2)
		price, _ := ratio_setting.GetModelPrice("MiniMax-H3", false)
		require.Equal(t, "per_request", video.Pricing.Unit)
		require.InDelta(t, price*group, *video.Pricing.RequestUSD, 1e-9)
		require.Equal(t, scaledNote, video.Pricing.Note, "MiniMax-H3 is billed per second, so the base price must say so")

		speech := get("tts-1")
		require.Equal(t, []string{purposeSpeech}, speech.Purposes)
		require.Equal(t, apiBase+"/v1/audio/speech", speech.Endpoints[0].URL)
		require.Contains(t, speech.Endpoints[0].Params, `"voice"`)

		other, res := call[getModelOutput](t, gw.connect(t, gw.keys["video"]), "get_model", map[string]any{"id": "gpt-4o-mini"})
		require.False(t, res.IsError)
		require.Equal(t, "no", other.Available)

		_, res = call[getModelOutput](t, s, "get_model", map[string]any{"id": "no-such-model"})
		require.True(t, res.IsError)
	})
}

// TestMCP_CheckKeyReportsLimitBalanceAndPayer checks the rest of check_key: a
// key's own limit and expiry, its whitelist, the personal balance (and the
// warning when it is empty), and that an organization key shows no balance -
// the company wallet is the owner's and admins' to see.
func TestMCP_CheckKeyReportsLimitBalanceAndPayer(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)

		limited, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["limited"]), "check_key", nil)
		require.Equal(t, statusOK, limited.Status)
		require.Equal(t, 0.5, *limited.KeyRemainingUSD)
		require.Equal(t, "$0.5 left of this key's own limit", limited.KeyLimit)
		require.NotEqual(t, "never", limited.ExpiresAt)
		require.Equal(t, "personal account", limited.PaidBy)
		require.Equal(t, 2.0, *limited.BalanceUSD)
		require.Equal(t, "default", limited.Group)

		video, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["video"]), "check_key", nil)
		require.Equal(t, []string{"MiniMax-H3"}, video.ModelWhitelist)
		require.Equal(t, "unlimited", video.KeyLimit)
		require.Equal(t, "never", video.ExpiresAt)

		broke, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["broke"]), "check_key", nil)
		require.Equal(t, statusOK, broke.Status)
		require.Equal(t, 0.0, *broke.BalanceUSD)
		require.Contains(t, broke.Message, walletURL)

		org, _ := call[checkKeyOutput](t, gw.connect(t, gw.keys["org"]), "check_key", nil)
		require.Equal(t, statusOK, org.Status)
		require.Equal(t, "organization", org.PaidBy)
		require.Nil(t, org.BalanceUSD)
	})
}

// TestMCP_TheKeyIsNeverInAnAnswer checks that no tool repeats the caller's key,
// whole or without its "sk-".
func TestMCP_TheKeyIsNeverInAnAnswer(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		key := gw.keys["limited"]
		s := gw.connect(t, key)
		for tool, args := range map[string]map[string]any{
			"list_models":           nil,
			"get_model":             {"id": "gpt-4o-mini"},
			"get_connection_config": {"client": "claude-code"},
			"check_key":             nil,
			"get_started":           nil,
		} {
			res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
			require.NoError(t, err)
			raw, err := common.Marshal(res)
			require.NoError(t, err)
			require.NotContains(t, string(raw), strings.TrimPrefix(key, "sk-"), tool)
		}
	})
}

// TestMCP_AModelHiddenFromTheCatalogIsStillPriced covers a model switched off
// on the Models page while its channel still serves it: the pricing catalog
// leaves it out, /v1/models and the relay do not. A key that can call it is
// told its real price, from the settings the relay bills by; a visitor does
// not learn it exists.
func TestMCP_AModelHiddenFromTheCatalogIsStillPriced(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		gw := newGateway(t, db)
		require.NoError(t, db.Create(&model.Model{ModelName: "MiniMax-H3"}).Error)
		require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "MiniMax-H3").Update("status", 0).Error)

		key := gw.keys["all"]
		require.Contains(t, gw.v1Models(t, key), "MiniMax-H3")
		video, res := call[getModelOutput](t, gw.connect(t, key), "get_model", map[string]any{"id": "MiniMax-H3"})
		require.False(t, res.IsError)
		require.Equal(t, "yes", video.Available)
		price, _ := ratio_setting.GetModelPrice("MiniMax-H3", false)
		require.Equal(t, "per_request", video.Pricing.Unit)
		require.InDelta(t, price*ratio_setting.GetGroupRatio("default"), *video.Pricing.RequestUSD, 1e-9)

		visitor := gw.connect(t, "")
		list, _ := call[listModelsOutput](t, visitor, "list_models", nil)
		require.NotContains(t, ids(list.Models), "MiniMax-H3")
		_, res = call[getModelOutput](t, visitor, "get_model", map[string]any{"id": "MiniMax-H3"})
		require.True(t, res.IsError)
	})
}
