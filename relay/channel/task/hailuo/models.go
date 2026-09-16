package hailuo

import "strings"

// IsV2Model reports whether the model is served by the v2 API (Hailuo 3.0 family).
func IsV2Model(model string) bool {
	return strings.HasPrefix(model, "MiniMax-H3")
}

type SubjectReference struct {
	Type  string   `json:"type"`  // Subject type, currently only supports "character"
	Image []string `json:"image"` // Array of subject reference images (currently only supports single image)
}

type VideoRequest struct {
	Model            string             `json:"model"`
	Prompt           string             `json:"prompt,omitempty"`
	PromptOptimizer  *bool              `json:"prompt_optimizer,omitempty"`
	FastPretreatment *bool              `json:"fast_pretreatment,omitempty"`
	Duration         *int               `json:"duration,omitempty"`
	Resolution       string             `json:"resolution,omitempty"`
	CallbackURL      string             `json:"callback_url,omitempty"`
	AigcWatermark    *bool              `json:"aigc_watermark,omitempty"`
	FirstFrameImage  string             `json:"first_frame_image,omitempty"` // For image-to-video and start-end-to-video
	LastFrameImage   string             `json:"last_frame_image,omitempty"`  // For start-end-to-video
	SubjectReference []SubjectReference `json:"subject_reference,omitempty"` // For subject-reference-to-video
}

type VideoResponse struct {
	TaskID   string   `json:"task_id"`
	BaseResp BaseResp `json:"base_resp"`
}

type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type QueryTaskRequest struct {
	TaskID string `json:"task_id"`
}

type QueryTaskResponse struct {
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	FileID      string   `json:"file_id,omitempty"`
	VideoWidth  int      `json:"video_width,omitempty"`
	VideoHeight int      `json:"video_height,omitempty"`
	BaseResp    BaseResp `json:"base_resp"`
}

type ErrorInfo struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type TaskStatusInfo struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	FileID    string `json:"file_id,omitempty"`
	VideoURL  string `json:"video_url,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

type ModelConfig struct {
	Name                 string
	DefaultResolution    string
	SupportedDurations   []int
	SupportedResolutions []string
	HasPromptOptimizer   bool
	HasFastPretreatment  bool
}

// ---------------------------------------------------------------------------
// v2 API shapes (Hailuo 3.0 family) — multimodal content array request,
// bare task_id response, {"task": {...}} query wrapper.
// ---------------------------------------------------------------------------

// V2MediaURL wraps a media URL inside a v2 content item.
type V2MediaURL struct {
	URL string `json:"url"`
}

// V2ContentItem is one multimodal input item in a v2 video generation request.
type V2ContentItem struct {
	Type     string      `json:"type"` // text | image_url | video_url | audio_url
	Text     string      `json:"text,omitempty"`
	ImageURL *V2MediaURL `json:"image_url,omitempty"`
	Role     string      `json:"role,omitempty"` // first_frame | last_frame | reference_image | ...
}

// V2VideoRequest is the request body for POST /v2/video_generation.
type V2VideoRequest struct {
	Model       string          `json:"model"`
	Content     []V2ContentItem `json:"content"`
	Resolution  string          `json:"resolution,omitempty"`
	Duration    *int            `json:"duration,omitempty"`
	Ratio       string          `json:"ratio,omitempty"`
	CallbackURL string          `json:"callback_url,omitempty"`
}

// V2Error is the v2 error object ({"type":"error","error":{...}}).
type V2Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// V2SubmitResponse is the response of POST /v2/video_generation.
type V2SubmitResponse struct {
	TaskID string   `json:"task_id"`
	Type   string   `json:"type,omitempty"`
	Error  *V2Error `json:"error,omitempty"`
}

// V2TaskContent holds the time-limited download URL of a finished v2 task.
type V2TaskContent struct {
	URL string `json:"url"`
}

// V2Task is the task object inside a v2 query response.
type V2Task struct {
	ID         string         `json:"id"`
	Model      string         `json:"model,omitempty"`
	Status     string         `json:"status"`
	Content    *V2TaskContent `json:"content,omitempty"`
	Resolution string         `json:"resolution,omitempty"`
	Duration   int            `json:"duration,omitempty"`
}

// V2QueryResponse is the response of GET /v2/query/video_generation/{task_id}.
type V2QueryResponse struct {
	Task  V2Task   `json:"task"`
	Type  string   `json:"type,omitempty"`
	Error *V2Error `json:"error,omitempty"`
}

type RetrieveFileResponse struct {
	File     FileObject `json:"file"`
	BaseResp BaseResp   `json:"base_resp"`
}

type FileObject struct {
	FileID      int64  `json:"file_id"`
	Bytes       int64  `json:"bytes"`
	CreatedAt   int64  `json:"created_at"`
	Filename    string `json:"filename"`
	Purpose     string `json:"purpose"`
	DownloadURL string `json:"download_url"`
}

func GetModelConfig(model string) ModelConfig {
	configs := map[string]ModelConfig{
		"MiniMax-H3": {
			Name:                 "MiniMax-H3",
			DefaultResolution:    Resolution2K,
			SupportedDurations:   []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SupportedResolutions: []string{Resolution768P, Resolution2K},
			HasPromptOptimizer:   false,
			HasFastPretreatment:  false,
		},
		"MiniMax-Hailuo-2.3": {
			Name:                 "MiniMax-Hailuo-2.3",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"MiniMax-Hailuo-2.3-Fast": {
			Name:                 "MiniMax-Hailuo-2.3-Fast",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"MiniMax-Hailuo-02": {
			Name:                 "MiniMax-Hailuo-02",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution512P, Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"T2V-01-Director": {
			Name:                 "T2V-01-Director",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"T2V-01": {
			Name:                 "T2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01-Director": {
			Name:                 "I2V-01-Director",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01-live": {
			Name:                 "I2V-01-live",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01": {
			Name:                 "I2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"S2V-01": {
			Name:                 "S2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
	}

	if config, exists := configs[model]; exists {
		return config
	}

	return ModelConfig{
		Name:                 model,
		DefaultResolution:    DefaultResolution,
		SupportedDurations:   []int{6},
		SupportedResolutions: []string{DefaultResolution},
		HasPromptOptimizer:   true,
		HasFastPretreatment:  false,
	}
}
