package admin

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IQTestHandler 模型智力测试管理接口（fork 扩展）。
type IQTestHandler struct {
	svc *service.IQTestService
}

func NewIQTestHandler(svc *service.IQTestService) *IQTestHandler {
	return &IQTestHandler{svc: svc}
}

func iqRespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrIQQuestionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrIQInvalidInput):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, err.Error())
	}
}

func iqParamID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}

// ---------------- questions ----------------

// ListQuestions GET /admin/iq/questions
func (h *IQTestHandler) ListQuestions(c *gin.Context) {
	qs, err := h.svc.ListQuestions(c.Request.Context())
	if err != nil {
		iqRespondError(c, err)
		return
	}
	stats, _ := h.svc.SampleStats(c.Request.Context())
	response.Success(c, gin.H{
		"questions":              qs,
		"sample_stats":           stats,
		"min_reference_samples":  service.IQMinReferenceSamples,
		"recommended_samples":    service.IQRecommendedReferenceSamples,
		"min_classifier_samples": service.IQMinClassifierSamples,
	})
}

type iqSaveQuestionRequest struct {
	Slug     string   `json:"slug" binding:"required"`
	Title    string   `json:"title"`
	Prompt   string   `json:"prompt" binding:"required"`
	Keywords []string `json:"keywords"`
	Enabled  *bool    `json:"enabled"`
}

// SaveQuestion POST /admin/iq/questions
func (h *IQTestHandler) SaveQuestion(c *gin.Context) {
	var req iqSaveQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	q, err := h.svc.SaveQuestion(c.Request.Context(), &service.IQQuestion{Slug: req.Slug, Title: req.Title, Prompt: req.Prompt, Keywords: req.Keywords, Enabled: enabled})
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, q)
}

// DeleteQuestion DELETE /admin/iq/questions/:slug
func (h *IQTestHandler) DeleteQuestion(c *gin.Context) {
	if err := h.svc.DeleteQuestion(c.Request.Context(), c.Param("slug")); err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ---------------- run ----------------

type iqRunRequest struct {
	ModelID       string `json:"model_id"`
	QuestionSlug  string `json:"question_slug" binding:"required"`
	ParallelCount int    `json:"parallel_count"`
}

// RunAccount POST /admin/accounts/:id/iq-test  手动测试（同步返回整轮结果）
func (h *IQTestHandler) RunAccount(c *gin.Context) {
	accountID, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	var req iqRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	round, err := h.svc.RunRound(c.Request.Context(), service.IQRunRequest{
		AccountID: accountID, ModelID: strings.TrimSpace(req.ModelID), QuestionSlug: req.QuestionSlug, ParallelCount: req.ParallelCount,
	}, nil)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, round)
}

// ListResults GET /admin/iq/results?account_id=&plan_id=&limit=
func (h *IQTestHandler) ListResults(c *gin.Context) {
	accountID, _ := strconv.ParseInt(c.Query("account_id"), 10, 64)
	planID, _ := strconv.ParseInt(c.Query("plan_id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	results, err := h.svc.ListResults(c.Request.Context(), service.IQResultFilter{AccountID: accountID, PlanID: planID, Limit: limit})
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, results)
}

type iqLabelRequest struct {
	Label string `json:"label" binding:"required"`
}

// LabelResult POST /admin/iq/results/:id/label
func (h *IQTestHandler) LabelResult(c *gin.Context) {
	id, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	var req iqLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.LabelResult(c.Request.Context(), id, req.Label); err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, gin.H{"labeled": true})
}

// ---------------- plans ----------------

type iqPlanRequest struct {
	ModelID        string `json:"model_id"`
	QuestionSlug   string `json:"question_slug" binding:"required"`
	ParallelCount  int    `json:"parallel_count"`
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
	FailAction     string `json:"fail_action"`
	FailThreshold  int    `json:"fail_threshold"`
	MaxResults     int    `json:"max_results"`
}

func (r *iqPlanRequest) toPlan() *service.IQTestPlan {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return &service.IQTestPlan{
		ModelID: strings.TrimSpace(r.ModelID), QuestionSlug: r.QuestionSlug, ParallelCount: r.ParallelCount,
		CronExpression: r.CronExpression, Enabled: enabled, FailAction: r.FailAction, FailThreshold: r.FailThreshold,
		MaxResults: r.MaxResults,
	}
}

// ListPlans GET /admin/accounts/:id/iq-plans
func (h *IQTestHandler) ListPlans(c *gin.Context) {
	accountID, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	plans, err := h.svc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, plans)
}

// CreatePlan POST /admin/accounts/:id/iq-plans
func (h *IQTestHandler) CreatePlan(c *gin.Context) {
	accountID, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	var req iqPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p := req.toPlan()
	p.AccountID = accountID
	plan, err := h.svc.CreatePlan(c.Request.Context(), p)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, plan)
}

// UpdatePlan PUT /admin/iq/plans/:id
func (h *IQTestHandler) UpdatePlan(c *gin.Context) {
	id, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	var req iqPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p := req.toPlan()
	p.ID = id
	plan, err := h.svc.UpdatePlan(c.Request.Context(), p)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, plan)
}

// DeletePlan DELETE /admin/iq/plans/:id
func (h *IQTestHandler) DeletePlan(c *gin.Context) {
	id, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(c.Request.Context(), id); err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// RunPlan POST /admin/iq/plans/:id/run
func (h *IQTestHandler) RunPlan(c *gin.Context) {
	id, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	round, err := h.svc.RunPlanNow(c.Request.Context(), id)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, round)
}

// ---------------- reference samples ----------------

// ListSamples GET /admin/iq/questions/:slug/samples
func (h *IQTestHandler) ListSamples(c *gin.Context) {
	slug := c.Param("slug")
	samples, err := h.svc.ListSamples(c.Request.Context(), slug)
	if err != nil {
		iqRespondError(c, err)
		return
	}
	classifier, _ := h.svc.GetClassifier(c.Request.Context(), slug)
	response.Success(c, gin.H{"samples": samples, "classifier": classifier})
}

type iqSampleRequest struct {
	ModelID string `json:"model_id"`
	Label   string `json:"label"`
	Note    string `json:"note"`
	Content string `json:"content"`
}

const iqMaxSampleBytes = 2 << 20

// UploadSamples POST /admin/iq/questions/:slug/samples
// 支持 JSON {content,model_id,label,note} 或 multipart（字段 files 可多个，另带 model_id/label）。
func (h *IQTestHandler) UploadSamples(c *gin.Context) {
	slug := c.Param("slug")
	ctx := c.Request.Context()
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		form, err := c.MultipartForm()
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		modelID := c.PostForm("model_id")
		label := c.PostForm("label")
		var created []*service.IQReferenceSample
		var failed []gin.H
		for _, fh := range form.File["files"] {
			if fh.Size > iqMaxSampleBytes {
				failed = append(failed, gin.H{"file": fh.Filename, "error": "file too large"})
				continue
			}
			f, err := fh.Open()
			if err != nil {
				failed = append(failed, gin.H{"file": fh.Filename, "error": err.Error()})
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, iqMaxSampleBytes))
			_ = f.Close()
			if err != nil {
				failed = append(failed, gin.H{"file": fh.Filename, "error": err.Error()})
				continue
			}
			smp, err := h.svc.AddSample(ctx, &service.IQReferenceSample{QuestionSlug: slug, ModelID: modelID, Label: label, Note: fh.Filename, SVG: string(data)})
			if err != nil {
				failed = append(failed, gin.H{"file": fh.Filename, "error": err.Error()})
				continue
			}
			created = append(created, smp)
		}
		response.Success(c, gin.H{"created": created, "failed": failed})
		return
	}
	var req iqSampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(req.Content) > iqMaxSampleBytes {
		response.BadRequest(c, "content too large")
		return
	}
	smp, err := h.svc.AddSample(ctx, &service.IQReferenceSample{QuestionSlug: slug, ModelID: req.ModelID, Label: req.Label, Note: req.Note, SVG: req.Content})
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, gin.H{"created": []*service.IQReferenceSample{smp}, "failed": []gin.H{}})
}

// DeleteSample DELETE /admin/iq/questions/:slug/samples/:id
func (h *IQTestHandler) DeleteSample(c *gin.Context) {
	id, ok := iqParamID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteSample(c.Request.Context(), c.Param("slug"), id); err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// RetrainClassifier POST /admin/iq/questions/:slug/classifier/retrain
func (h *IQTestHandler) RetrainClassifier(c *gin.Context) {
	info, err := h.svc.RetrainClassifier(c.Request.Context(), c.Param("slug"))
	if err != nil {
		iqRespondError(c, err)
		return
	}
	response.Success(c, info)
}
