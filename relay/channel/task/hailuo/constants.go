package hailuo

const (
	ChannelName = "hailuo-video"
)

var ModelList = []string{
	"MiniMax-H3",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	"T2V-01-Director",
	"T2V-01",
	"I2V-01-Director",
	"I2V-01-live",
	"I2V-01",
	"S2V-01",
}

const (
	TextToVideoEndpoint = "/v1/video_generation"
	QueryTaskEndpoint   = "/v1/query/video_generation"

	// The Hailuo 3.0 family (intl platform, API host api.minimax.io) is served
	// by the v2 API: submit POST /v2/video_generation, poll
	// GET /v2/query/video_generation/{task_id} (path param, not query string).
	V2TextToVideoEndpoint = "/v2/video_generation"
	V2QueryTaskEndpoint   = "/v2/query/video_generation"
)

// v2UpstreamTaskIDPrefix tags stored upstream task ids that must be polled on
// the v2 query endpoint; FetchTask strips it. v1 task ids are stored bare, so
// existing tasks keep polling the v1 endpoint.
const v2UpstreamTaskIDPrefix = "v2:"

const (
	StatusSuccess    = 0
	StatusRateLimit  = 1002
	StatusAuthFailed = 1004
	StatusNoBalance  = 1008
	StatusSensitive  = 1026
	StatusParamError = 2013
	StatusInvalidKey = 2049
)

const (
	TaskStatusPreparing  = "Preparing"
	TaskStatusQueueing   = "Queueing"
	TaskStatusProcessing = "Processing"
	TaskStatusSuccess    = "Success"
	TaskStatusFailed     = "Fail"
)

// v2 task statuses (lowercase, unlike the capitalized v1 statuses above).
const (
	V2TaskStatusQueued    = "queued"
	V2TaskStatusRunning   = "running"
	V2TaskStatusSucceeded = "succeeded"
	V2TaskStatusFailed    = "failed"
	V2TaskStatusCancelled = "cancelled"
)

const (
	Resolution512P  = "512P"
	Resolution720P  = "720P"
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
	Resolution2K    = "2K"
)

const (
	DefaultDuration   = 6
	DefaultResolution = Resolution720P
)

// MiniMax-H3 duration bounds in integer seconds (intl API docs).
const (
	H3MinDuration = 4
	H3MaxDuration = 15
)

// MiniMax-H3 upstream per-OUTPUT-SECOND rates in USD (platform.minimax.io
// pricing, checked 2026-09-15). The 2K rate must equal the MiniMax-H3 row in
// setting/ratio_setting defaultModelPrice (guarded by a test); EstimateBilling
// derives the 768P discount ratio from these two.
// ⚠️ Rates are at cost — margin pending Q3 (docs/video-first-wave-prd.md §10).
const (
	H3RatePerSecond768P = 0.08
	H3RatePerSecond2K   = 0.13
)
