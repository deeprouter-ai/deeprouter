package hailuo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
)

// https://platform.minimaxi.com/docs/api-reference/video-generation-intro
type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if IsV2Model(info.UpstreamModelName) {
		return fmt.Sprintf("%s%s", a.baseURL, V2TextToVideoEndpoint), nil
	}
	return fmt.Sprintf("%s%s", a.baseURL, TextToVideoEndpoint), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	var body any
	var err error
	if IsV2Model(info.UpstreamModelName) {
		body, err = a.convertToV2RequestPayload(&req, info)
	} else {
		body, err = a.convertToRequestPayload(&req, info)
	}
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	if IsV2Model(info.UpstreamModelName) {
		return a.doV2Response(c, responseBody, info)
	}

	var hResp VideoResponse
	if err := common.Unmarshal(responseBody, &hResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if hResp.BaseResp.StatusCode != StatusSuccess {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("hailuo api error: %s", hResp.BaseResp.StatusMsg),
			strconv.Itoa(hResp.BaseResp.StatusCode),
			http.StatusBadRequest,
		)
		return
	}

	writeSubmitResponse(c, info)
	return hResp.TaskID, responseBody, nil
}

// doV2Response parses a v2 submission response; the returned task id carries
// the v2 prefix so FetchTask later polls the v2 query endpoint.
func (a *TaskAdaptor) doV2Response(c *gin.Context, responseBody []byte, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	var v2Resp V2SubmitResponse
	if err := common.Unmarshal(responseBody, &v2Resp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	if v2Resp.Error != nil {
		return "", nil, service.TaskErrorWrapper(
			fmt.Errorf("hailuo api error: %s", v2Resp.Error.Message),
			v2Resp.Error.Type,
			http.StatusBadRequest,
		)
	}
	if v2Resp.TaskID == "" {
		return "", nil, service.TaskErrorWrapper(
			fmt.Errorf("hailuo api returned no task_id, body: %s", responseBody),
			"missing_task_id",
			http.StatusInternalServerError,
		)
	}

	writeSubmitResponse(c, info)
	return v2UpstreamTaskIDPrefix + v2Resp.TaskID, responseBody, nil
}

// writeSubmitResponse writes the OpenAI-style video object for a successful submission.
func writeSubmitResponse(c *gin.Context, info *relaycommon.RelayInfo) {
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	var uri string
	if id, isV2 := strings.CutPrefix(taskID, v2UpstreamTaskIDPrefix); isV2 {
		uri = fmt.Sprintf("%s%s/%s", baseUrl, V2QueryTaskEndpoint, id)
	} else {
		uri = fmt.Sprintf("%s%s?task_id=%s", baseUrl, QueryTaskEndpoint, taskID)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*VideoRequest, error) {
	modelConfig := GetModelConfig(info.UpstreamModelName)
	duration := DefaultDuration
	if req.Duration > 0 {
		duration = req.Duration
	}
	resolution := modelConfig.DefaultResolution
	if req.Size != "" {
		resolution = a.parseResolutionFromSize(req.Size, modelConfig)
	}

	videoRequest := &VideoRequest{
		Model:      info.UpstreamModelName,
		Prompt:     req.Prompt,
		Duration:   &duration,
		Resolution: resolution,
	}
	if err := req.UnmarshalMetadata(&videoRequest); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata to video request failed")
	}

	return videoRequest, nil
}

func (a *TaskAdaptor) parseResolutionFromSize(size string, modelConfig ModelConfig) string {
	switch {
	case strings.Contains(size, "1080"):
		return Resolution1080P
	case strings.Contains(size, "768"):
		return Resolution768P
	case strings.Contains(size, "720"):
		return Resolution720P
	case strings.Contains(size, "512"):
		return Resolution512P
	default:
		return modelConfig.DefaultResolution
	}
}

// EstimateBilling returns duration/resolution OtherRatios for v2 (H3) models,
// whose defaultModelPrice entry is USD per output second at 2K; v1 models keep
// their flat per-call price (nil ratios).
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if !IsV2Model(info.OriginModelName) {
		return nil
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	duration, resolution := resolveV2DurationResolution(&req, GetModelConfig(info.OriginModelName))
	ratios := map[string]float64{"seconds": float64(duration)}
	if resolution == Resolution768P {
		ratios["resolution"] = H3RatePerSecond768P / H3RatePerSecond2K
	}
	return ratios
}

// resolveV2DurationResolution derives the billed duration and resolution for a
// v2 (H3) submission from the top-level request fields; the request payload is
// forced back to these values after the metadata merge, so metadata cannot
// widen what was billed.
func resolveV2DurationResolution(req *relaycommon.TaskSubmitReq, modelConfig ModelConfig) (int, string) {
	duration := DefaultDuration
	if req.Duration > 0 {
		duration = req.Duration
	}
	if duration < H3MinDuration {
		duration = H3MinDuration
	}
	if duration > H3MaxDuration {
		duration = H3MaxDuration
	}

	resolution := modelConfig.DefaultResolution
	if req.Size != "" {
		if s := strings.ToUpper(req.Size); strings.Contains(s, "2K") {
			resolution = Resolution2K
		} else if strings.Contains(s, "768") {
			resolution = Resolution768P
		}
	}
	return duration, resolution
}

// convertToV2RequestPayload maps the OpenAI-style submit request onto the v2
// multimodal body used by the Hailuo 3.0 family.
func (a *TaskAdaptor) convertToV2RequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*V2VideoRequest, error) {
	duration, resolution := resolveV2DurationResolution(req, GetModelConfig(info.UpstreamModelName))

	content := []V2ContentItem{{Type: "text", Text: req.Prompt}}
	firstFrame := req.Image
	if firstFrame == "" && len(req.Images) > 0 {
		firstFrame = req.Images[0]
	}
	if firstFrame != "" {
		content = append(content, V2ContentItem{
			Type:     "image_url",
			ImageURL: &V2MediaURL{URL: firstFrame},
			Role:     "first_frame",
		})
	}

	videoRequest := &V2VideoRequest{
		Model:   info.UpstreamModelName,
		Content: content,
	}
	if err := req.UnmarshalMetadata(videoRequest); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata to video request failed")
	}
	// Billing was pre-estimated from the top-level fields (EstimateBilling runs
	// before the body is built); metadata may add ratio/callback_url etc. but
	// must not change what is billed. Duration is attached only after the merge —
	// unmarshalling into a non-nil *int would write through it.
	videoRequest.Model = info.UpstreamModelName
	videoRequest.Duration = &duration
	videoRequest.Resolution = resolution

	return videoRequest, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	// v2 query responses wrap the task ({"task":{...}}) and carry no base_resp.
	var v2Resp V2QueryResponse
	if err := common.Unmarshal(respBody, &v2Resp); err == nil && (v2Resp.Task.Status != "" || v2Resp.Error != nil) {
		return parseV2TaskResult(&v2Resp), nil
	}

	resTask := QueryTaskResponse{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{}

	if resTask.BaseResp.StatusCode == StatusSuccess {
		taskResult.Code = 0
	} else {
		taskResult.Code = resTask.BaseResp.StatusCode
		taskResult.Reason = resTask.BaseResp.StatusMsg
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
	}

	switch resTask.Status {
	case TaskStatusPreparing, TaskStatusQueueing, TaskStatusProcessing:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
		if resTask.Status == TaskStatusProcessing {
			taskResult.Progress = "50%"
		}
	case TaskStatusSuccess:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = a.buildVideoURL(resTask.TaskID, resTask.FileID)
	case TaskStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// parseV2TaskResult maps a v2 query response onto the common TaskInfo shape;
// the video URL comes directly in the response (no file retrieve step).
func parseV2TaskResult(resp *V2QueryResponse) *relaycommon.TaskInfo {
	taskResult := &relaycommon.TaskInfo{}

	switch resp.Task.Status {
	case V2TaskStatusQueued:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "20%"
	case V2TaskStatusRunning:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case V2TaskStatusSucceeded:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if resp.Task.Content != nil {
			taskResult.Url = resp.Task.Content.URL
		}
	case V2TaskStatusFailed, V2TaskStatusCancelled:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task " + resp.Task.Status
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	if resp.Error != nil {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resp.Error.Message
	}
	return taskResult
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	// v2 task data has no base_resp; surface its error object if present.
	var v2Resp V2QueryResponse
	if err := common.Unmarshal(originTask.Data, &v2Resp); err == nil && (v2Resp.Task.Status != "" || v2Resp.Error != nil) {
		openAIVideo := originTask.ToOpenAIVideo()
		if v2Resp.Error != nil {
			openAIVideo.Error = &dto.OpenAIVideoError{
				Message: v2Resp.Error.Message,
				Code:    v2Resp.Error.Type,
			}
		}
		jsonData, err := common.Marshal(openAIVideo)
		if err != nil {
			return nil, errors.Wrap(err, "marshal openai video failed")
		}
		return jsonData, nil
	}

	var hailuoResp QueryTaskResponse
	if err := common.Unmarshal(originTask.Data, &hailuoResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal hailuo task data failed")
	}

	openAIVideo := originTask.ToOpenAIVideo()
	if hailuoResp.BaseResp.StatusCode != StatusSuccess {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: hailuoResp.BaseResp.StatusMsg,
			Code:    strconv.Itoa(hailuoResp.BaseResp.StatusCode),
		}
	}

	jsonData, err := common.Marshal(openAIVideo)
	if err != nil {
		return nil, errors.Wrap(err, "marshal openai video failed")
	}

	return jsonData, nil
}

func (a *TaskAdaptor) buildVideoURL(_, fileID string) string {
	if a.apiKey == "" || a.baseURL == "" {
		return ""
	}

	url := fmt.Sprintf("%s/v1/files/retrieve?file_id=%s", a.baseURL, fileID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var retrieveResp RetrieveFileResponse
	if err := common.Unmarshal(responseBody, &retrieveResp); err != nil {
		return ""
	}

	if retrieveResp.BaseResp.StatusCode != StatusSuccess {
		return ""
	}

	return retrieveResp.File.DownloadURL
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
