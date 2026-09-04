package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newRateLimitTestContext builds a gin.Context whose ClientIP() is fully
// controlled via RemoteAddr, so each test case can use a distinct key and
// avoid bleeding state through the shared package-level inMemoryRateLimiter.
func newRateLimitTestContext(remoteAddr string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	c.Request = req
	return c, w
}

func TestGlobalWebRateLimit_DisabledIsNoOp(t *testing.T) {
	oldEnable := common.GlobalWebRateLimitEnable
	oldRedis := common.RedisEnabled
	common.GlobalWebRateLimitEnable = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.GlobalWebRateLimitEnable = oldEnable
		common.RedisEnabled = oldRedis
	})

	limiter := GlobalWebRateLimit()
	c, _ := newRateLimitTestContext("10.0.0.1:1234")
	limiter(c)

	assert.False(t, c.IsAborted(), "the request must not be aborted when the rate limit is disabled")
	assert.NotEqual(t, http.StatusTooManyRequests, c.Writer.Status())
}

func TestGlobalWebRateLimit_InMemoryBlocksAfterThreshold(t *testing.T) {
	oldEnable := common.GlobalWebRateLimitEnable
	oldRedis := common.RedisEnabled
	oldNum := common.GlobalWebRateLimitNum
	oldDuration := common.GlobalWebRateLimitDuration
	common.GlobalWebRateLimitEnable = true
	common.RedisEnabled = false
	common.GlobalWebRateLimitNum = 2
	common.GlobalWebRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalWebRateLimitEnable = oldEnable
		common.RedisEnabled = oldRedis
		common.GlobalWebRateLimitNum = oldNum
		common.GlobalWebRateLimitDuration = oldDuration
	})

	const ip = "10.0.0.2:1234"

	for i := 0; i < 2; i++ {
		limiter := GlobalWebRateLimit()
		c, _ := newRateLimitTestContext(ip)
		limiter(c)

		require.False(t, c.IsAborted(), "request %d within the limit must not be aborted", i+1)
		assert.NotEqual(t, http.StatusTooManyRequests, c.Writer.Status())
	}

	limiter := GlobalWebRateLimit()
	c, _ := newRateLimitTestContext(ip)
	limiter(c)

	assert.True(t, c.IsAborted(), "request past the limit must be aborted")
	assert.Equal(t, http.StatusTooManyRequests, c.Writer.Status())
}
