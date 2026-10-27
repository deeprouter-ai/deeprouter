package mcp

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// What check_key can say about a key.
const (
	statusOK        = "ok"
	statusNoKey     = "no_key"    // the request carried no key
	statusInvalid   = "invalid"   // no such key
	statusDisabled  = "disabled"  // the key, or the account behind it, is switched off
	statusExpired   = "expired"   // past its expiry time
	statusExhausted = "exhausted" // the key's own spending limit is used up
	statusRefused   = "refused"   // the gateway turned it away for another reason; Message says which
	statusError     = "error"     // the key could not be looked up right now
)

// callerKey is the context key the caller is stored under.
type callerKey struct{}

// caller is whoever sent one MCP request. Its key is looked at only when a
// tool needs it, and then only once, so initialize, tools/list and
// notifications never touch the database.
type caller struct {
	request *http.Request // a copy without the body, safe to read from a tool's goroutine
	engine  *gin.Engine
	once    sync.Once
	state   keyState
}

// keyState is what checking a caller's key found.
type keyState struct {
	Status  string
	Message string // the gateway's own words, when it refused the key
	Token   *model.Token
	auth    *gin.Context // the key's authenticated context; set only when Status is ok
}

// newCaller remembers the request for the tools that will run on it.
func newCaller(r *http.Request, engine *gin.Engine) *caller {
	copied := r.Clone(context.Background())
	copied.Body = http.NoBody
	return &caller{request: copied, engine: engine}
}

// withCaller puts the caller where the tools will find it. The SDK hands a
// tool a context carrying the HTTP request's values.
func withCaller(ctx context.Context, c *caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// callerFrom returns the request's caller. A tool reached without NewHandler
// in front of it gets one with no key.
func callerFrom(ctx context.Context) *caller {
	if c, ok := ctx.Value(callerKey{}).(*caller); ok {
		return c
	}
	c := &caller{}
	c.once.Do(func() { c.state = keyState{Status: statusNoKey} })
	return c
}

// key checks the caller's key the first time it is asked and returns the
// result every time after.
func (c *caller) key() *keyState {
	c.once.Do(func() { c.state = c.check() })
	return &c.state
}

// check decides what the key is. A key that is fine by its own record is then
// put through middleware.TokenAuth itself, so anything that would refuse it
// on /v1/models - a client IP outside its list, a group it may not use, an
// organization without a wallet - refuses it here too, in the same words.
func (c *caller) check() keyState {
	key := bearerKey(c.request.Header.Get("Authorization"))
	if key == "" {
		return keyState{Status: statusNoKey}
	}
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		// The name of a variable, not its value: the keys page's command
		// quotes `${ANTHROPIC_AUTH_TOKEN}` for the client to expand, and the
		// client found no such variable where it runs. That is no key at all,
		// named back as the unset variable - not as a key DeepRouter never
		// issued, which would send the user off to copy one again.
		return keyState{Status: statusNoKey, Message: key}
	}
	token, err := model.GetTokenByKey(key, false)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return keyState{Status: statusInvalid}
	}
	if err != nil {
		common.SysLog("mcp: key lookup failed: " + err.Error())
		return keyState{Status: statusError}
	}
	if status := tokenStatus(token, common.GetTimestamp()); status != statusOK {
		return keyState{Status: status, Token: token}
	}

	auth, out := c.shadow(nil)
	middleware.TokenAuth()(auth)
	if auth.IsAborted() {
		if user, err := model.GetUserCache(token.UserId); err == nil && user.Status != common.UserStatusEnabled {
			return keyState{Status: statusDisabled, Message: "the account that owns this key is disabled", Token: token}
		}
		return keyState{Status: statusRefused, Message: gatewayMessage(out.Bytes()), Token: token}
	}
	return keyState{Status: statusOK, Token: token, auth: auth}
}

// bearerKey takes the key out of an Authorization header the way
// middleware.TokenAuth does: "Bearer " and "sk-" are both optional, and
// whatever follows a "-" is not part of the key.
func bearerKey(header string) string {
	key := header
	if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	key = strings.TrimPrefix(key, "sk-")
	return strings.Split(key, "-")[0]
}

// tokenStatus reads a key's own record as model.ValidateUserToken judges it,
// but names the reason instead of folding every one into "invalid".
func tokenStatus(token *model.Token, now int64) string {
	switch {
	case token.Status == common.TokenStatusDisabled:
		return statusDisabled
	case token.Status == common.TokenStatusExpired || (token.ExpiredTime != -1 && token.ExpiredTime < now):
		return statusExpired
	case token.Status == common.TokenStatusExhausted || (!token.UnlimitedQuota && token.RemainQuota <= 0):
		return statusExhausted
	case token.Status != common.TokenStatusEnabled:
		return statusDisabled
	}
	return statusOK
}

// gatewayMessage reads the reason out of an error the gateway wrote.
func gatewayMessage(body []byte) string {
	var answer struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if common.Unmarshal(body, &answer) != nil {
		return ""
	}
	if answer.Error.Message != "" {
		return answer.Error.Message
	}
	return answer.Message
}

// keyModels lists the models the caller's key can use by running the
// /v1/models handler itself on the key's authenticated context, so the list
// can never drift from what the API answers.
func (c *caller) keyModels(state *keyState) ([]dto.OpenAIModels, error) {
	ctx, out := c.shadow(state.auth.Keys)
	controller.ListModels(ctx, constant.ChannelTypeOpenAI)
	var answer struct {
		Success bool               `json:"success"`
		Message string             `json:"message"`
		Data    []dto.OpenAIModels `json:"data"`
	}
	if err := common.Unmarshal(out.Bytes(), &answer); err != nil {
		return nil, err
	}
	if !answer.Success {
		return nil, errors.New(answer.Message)
	}
	return answer.Data, nil
}

// shadow builds a gin context for running one of the gateway's own handlers on
// this request without answering it: what the handler writes lands in the
// returned buffer. keys is copied in, so every run starts from the same state
// instead of sharing one.
func (c *caller) shadow(keys map[string]any) (*gin.Context, *bytes.Buffer) {
	out := &recorder{header: http.Header{}}
	ctx := gin.CreateTestContextOnly(out, c.engine)
	ctx.Request = c.request
	for k, v := range keys {
		ctx.Set(k, v)
	}
	return ctx, &out.body
}

// recorder is the response writer of a shadow context.
type recorder struct {
	header http.Header
	body   bytes.Buffer
}

// Header returns the headers the handler set, which nobody reads.
func (r *recorder) Header() http.Header { return r.header }

// Write keeps what the handler wrote.
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// WriteHeader drops the status: the body says everything the tools need.
func (r *recorder) WriteHeader(int) {}
