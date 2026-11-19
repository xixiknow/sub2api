package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// 模型智力测试服务（fork 扩展）：手动 / 定时对账号出题、自动判分，并按结果处置。

const (
	IQVerdictPassed       = "passed"
	IQVerdictFailed       = "failed"
	IQVerdictInconclusive = "inconclusive"
	IQVerdictError        = "error"

	IQActionObserveOnly       = "observe_only"
	IQActionHealthPenalty     = "health_penalty"
	IQActionDisableScheduling = "disable_scheduling"

	IQRandomQuestionSlug = "random"
	IQCandyQuestionSlug  = "candy"

	iqMaxParallel        = 8
	iqSampleTimeout      = 6 * time.Minute
	iqAnswerMaxTokens    = 8192
	iqSVGMaxTokens       = 16000
	iqHealthPenaltyValue = 0.2
	iqMaxResponseStore   = 512 * 1024
	iqRunnerWorkers      = 4
)

var (
	ErrIQQuestionNotFound = errors.New("iq question not found")
	ErrIQInvalidInput     = errors.New("invalid iq test input")
)

// IQQuestion 是题库中的一道题。
type IQQuestion struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Prompt    string    `json:"prompt"`
	Keywords  []string  `json:"keywords"`
	Builtin   bool      `json:"builtin"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IQTestPlan 是账号的定时智力测试计划。
type IQTestPlan struct {
	ID               int64      `json:"id"`
	AccountID        int64      `json:"account_id"`
	ModelID          string     `json:"model_id"`
	QuestionSlug     string     `json:"question_slug"`
	ParallelCount    int        `json:"parallel_count"`
	CronExpression   string     `json:"cron_expression"`
	Enabled          bool       `json:"enabled"`
	FailAction       string     `json:"fail_action"`
	FailThreshold    int        `json:"fail_threshold"`
	ConsecutiveFails int        `json:"consecutive_fails"`
	ActionApplied    bool       `json:"action_applied"`
	MaxResults       int        `json:"max_results"`
	LastVerdict      string     `json:"last_verdict"`
	LastRunAt        *time.Time `json:"last_run_at"`
	NextRunAt        *time.Time `json:"next_run_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// IQTestResult 是一次出题（一个样本）的结果。
type IQTestResult struct {
	ID           int64          `json:"id"`
	PlanID       *int64         `json:"plan_id,omitempty"`
	AccountID    int64          `json:"account_id"`
	ModelID      string         `json:"model_id"`
	QuestionSlug string         `json:"question_slug"`
	RoundID      string         `json:"round_id"`
	Prompt       string         `json:"prompt"`
	ResponseText string         `json:"response_text"`
	Verdict      string         `json:"verdict"`
	Score        *float64       `json:"score,omitempty"`
	Detail       map[string]any `json:"detail"`
	ErrorMessage string         `json:"error_message"`
	Label        string         `json:"label"`
	LatencyMs    int64          `json:"latency_ms"`
	CreatedAt    time.Time      `json:"created_at"`
}

// IQReferenceSample 是画图题的参考样本。
type IQReferenceSample struct {
	ID           int64              `json:"id"`
	QuestionSlug string             `json:"question_slug"`
	ModelID      string             `json:"model_id"`
	Label        string             `json:"label"`
	Source       string             `json:"source"`
	Note         string             `json:"note"`
	SVG          string             `json:"svg,omitempty"`
	Features     map[string]float64 `json:"features,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
}

// IQSampleStat 是参考样本数量统计。
type IQSampleStat struct {
	QuestionSlug string `json:"question_slug"`
	ModelID      string `json:"model_id"`
	Label        string `json:"label"`
	Count        int    `json:"count"`
}

// IQClassifierInfo 是已训练分类器的元信息。
type IQClassifierInfo struct {
	QuestionSlug string           `json:"question_slug"`
	Model        *IQLogisticModel `json:"-"`
	NGood        int              `json:"n_good"`
	NBad         int              `json:"n_bad"`
	Accuracy     float64          `json:"accuracy"`
	TrainedAt    time.Time        `json:"trained_at"`
}

// IQResultFilter 是结果列表过滤条件。
type IQResultFilter struct {
	AccountID int64
	PlanID    int64
	Limit     int
}

// IQTestRepository 是智力测试的数据访问接口。
type IQTestRepository interface {
	UpsertBuiltinQuestion(ctx context.Context, q *IQQuestion) error
	ListQuestions(ctx context.Context) ([]*IQQuestion, error)
	GetQuestion(ctx context.Context, slug string) (*IQQuestion, error)
	SaveQuestion(ctx context.Context, q *IQQuestion) (*IQQuestion, error)
	DeleteQuestion(ctx context.Context, slug string) error

	CreatePlan(ctx context.Context, p *IQTestPlan) (*IQTestPlan, error)
	UpdatePlan(ctx context.Context, p *IQTestPlan) (*IQTestPlan, error)
	GetPlan(ctx context.Context, id int64) (*IQTestPlan, error)
	DeletePlan(ctx context.Context, id int64) error
	ListPlansByAccount(ctx context.Context, accountID int64) ([]*IQTestPlan, error)
	ListDuePlans(ctx context.Context, now time.Time) ([]*IQTestPlan, error)
	ListPenaltyPlans(ctx context.Context) ([]*IQTestPlan, error)
	UpdatePlanAfterRun(ctx context.Context, p *IQTestPlan) error

	CreateResult(ctx context.Context, r *IQTestResult) (*IQTestResult, error)
	GetResult(ctx context.Context, id int64) (*IQTestResult, error)
	ListResults(ctx context.Context, f IQResultFilter) ([]*IQTestResult, error)
	SetResultLabel(ctx context.Context, id int64, label string) error
	PruneResults(ctx context.Context, planID int64, keep int) error

	CreateSample(ctx context.Context, s *IQReferenceSample) (*IQReferenceSample, error)
	ListSamples(ctx context.Context, questionSlug string, withSVG bool) ([]*IQReferenceSample, error)
	DeleteSample(ctx context.Context, id int64) error
	SampleStats(ctx context.Context) ([]IQSampleStat, error)

	GetClassifier(ctx context.Context, questionSlug string) (*IQClassifierInfo, error)
	SaveClassifier(ctx context.Context, info *IQClassifierInfo) error
}

// IQRunRequest 是一次手动测试请求。
type IQRunRequest struct {
	AccountID     int64  `json:"account_id"`
	ModelID       string `json:"model_id"`
	QuestionSlug  string `json:"question_slug"`
	ParallelCount int    `json:"parallel_count"`
}

// IQRoundResult 是一轮测试（多个样本）的汇总。
type IQRoundResult struct {
	RoundID string          `json:"round_id"`
	Verdict string          `json:"verdict"`
	Results []*IQTestResult `json:"results"`
}

// IQTestService 智力测试服务。
type IQTestService struct {
	repo           IQTestRepository
	accountRepo    AccountRepository
	accountTestSvc *AccountTestService
	cfg            *config.Config
	health         *AccountHealthTracker

	refMu    sync.Mutex
	refCache map[string]*iqRefCacheEntry

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

type iqRefCacheEntry struct {
	set        *IQSVGReferenceSet
	classifier *IQLogisticModel
	builtAt    time.Time
}

// NewIQTestService 创建服务。
func NewIQTestService(repo IQTestRepository, accountRepo AccountRepository, accountTestSvc *AccountTestService, cfg *config.Config) *IQTestService {
	return &IQTestService{
		repo:           repo,
		accountRepo:    accountRepo,
		accountTestSvc: accountTestSvc,
		cfg:            cfg,
		health:         DefaultAccountHealthTracker(),
		refCache:       map[string]*iqRefCacheEntry{},
	}
}

// ---------------- 题库 ----------------

// SeedBuiltinQuestions 写入/更新内置题目。
func (s *IQTestService) SeedBuiltinQuestions(ctx context.Context) error {
	builtins := []*IQQuestion{
		{Slug: IQCandyQuestionSlug, Kind: IQKindCandy, Title: "糖果题（固定答案 21）", Prompt: iqCandyPrompt, Builtin: true, Enabled: true},
		{Slug: IQRandomQuestionSlug, Kind: IQKindRandomReasoning, Title: "随机推理题（每次生成新题）", Prompt: "", Builtin: true, Enabled: true},
	}
	for _, p := range iqBuiltinSVGQuestions {
		builtins = append(builtins, &IQQuestion{Slug: p.Slug, Kind: IQKindSVGDraw, Title: p.Title, Prompt: p.Prompt, Keywords: p.Keywords, Builtin: true, Enabled: true})
	}
	for _, q := range builtins {
		if err := s.repo.UpsertBuiltinQuestion(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *IQTestService) ListQuestions(ctx context.Context) ([]*IQQuestion, error) {
	return s.repo.ListQuestions(ctx)
}

// SaveQuestion 新增或更新自定义题目（目前仅支持 svg_draw 自定义）。
func (s *IQTestService) SaveQuestion(ctx context.Context, q *IQQuestion) (*IQQuestion, error) {
	q.Slug = strings.TrimSpace(strings.ToLower(q.Slug))
	q.Title = strings.TrimSpace(q.Title)
	q.Prompt = strings.TrimSpace(q.Prompt)
	if q.Slug == "" || len(q.Slug) > 64 || q.Prompt == "" {
		return nil, ErrIQInvalidInput
	}
	if existing, err := s.repo.GetQuestion(ctx, q.Slug); err == nil && existing != nil && existing.Builtin {
		return nil, fmt.Errorf("%w: builtin question cannot be modified", ErrIQInvalidInput)
	}
	q.Kind = IQKindSVGDraw
	q.Builtin = false
	return s.repo.SaveQuestion(ctx, q)
}

func (s *IQTestService) DeleteQuestion(ctx context.Context, slug string) error {
	q, err := s.repo.GetQuestion(ctx, slug)
	if err != nil {
		return err
	}
	if q.Builtin {
		return fmt.Errorf("%w: builtin question cannot be deleted", ErrIQInvalidInput)
	}
	return s.repo.DeleteQuestion(ctx, slug)
}

// ---------------- 执行与判分 ----------------

func newIQRoundID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}

func clampIQParallel(n int) int {
	if n < 1 {
		return 1
	}
	if n > iqMaxParallel {
		return iqMaxParallel
	}
	return n
}

// RunRound 执行一轮测试（parallelCount 个样本并发），保存结果并返回汇总。
func (s *IQTestService) RunRound(ctx context.Context, req IQRunRequest, planID *int64) (*IQRoundResult, error) {
	q, err := s.repo.GetQuestion(ctx, req.QuestionSlug)
	if err != nil || q == nil {
		return nil, ErrIQQuestionNotFound
	}
	n := clampIQParallel(req.ParallelCount)
	round := &IQRoundResult{RoundID: newIQRoundID(), Results: make([]*IQTestResult, n)}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					round.Results[i] = &IQTestResult{Verdict: IQVerdictError, ErrorMessage: fmt.Sprintf("panic: %v", r)}
				}
			}()
			round.Results[i] = s.runSample(ctx, q, req.AccountID, req.ModelID)
		}(i)
	}
	wg.Wait()
	for _, r := range round.Results {
		r.PlanID = planID
		r.AccountID = req.AccountID
		r.ModelID = req.ModelID
		r.QuestionSlug = q.Slug
		r.RoundID = round.RoundID
		if len(r.ResponseText) > iqMaxResponseStore {
			r.ResponseText = r.ResponseText[:iqMaxResponseStore]
		}
		if saved, err := s.repo.CreateResult(ctx, r); err == nil && saved != nil {
			r.ID = saved.ID
			r.CreatedAt = saved.CreatedAt
		} else if err != nil {
			logger.LegacyPrintf("service.iq_test", "[IQTest] save result failed: %v", err)
		}
	}
	round.Verdict = AggregateIQVerdict(round.Results)
	return round, nil
}

// AggregateIQVerdict 汇总一轮样本：有效样本全部通过→passed；失败样本≥有效样本一半→failed；
// 全部出错→error；其余→inconclusive。
func AggregateIQVerdict(results []*IQTestResult) string {
	passed, failed, valid := 0, 0, 0
	for _, r := range results {
		if r == nil {
			continue
		}
		switch r.Verdict {
		case IQVerdictPassed:
			passed++
			valid++
		case IQVerdictFailed:
			failed++
			valid++
		case IQVerdictInconclusive:
			valid++
		}
	}
	switch {
	case valid == 0:
		return IQVerdictError
	case passed == valid:
		return IQVerdictPassed
	case failed*2 >= valid:
		return IQVerdictFailed
	default:
		return IQVerdictInconclusive
	}
}

func (s *IQTestService) runSample(ctx context.Context, q *IQQuestion, accountID int64, modelID string) *IQTestResult {
	res := &IQTestResult{Detail: map[string]any{}}
	var prompt string
	var random *IQRandomQuestion
	maxTokens := iqAnswerMaxTokens
	switch q.Kind {
	case IQKindCandy:
		prompt = q.Prompt + iqAnswerSuffix
	case IQKindRandomReasoning:
		rq, err := GenerateIQRandomQuestion()
		if err != nil {
			res.Verdict = IQVerdictError
			res.ErrorMessage = err.Error()
			return res
		}
		random = rq
		prompt = rq.Prompt
		res.Detail["random_family"] = rq.Family
		res.Detail["random_id"] = rq.ID
		res.Detail["reference"] = rq.Reference
	case IQKindSVGDraw:
		prompt = q.Prompt + iqSVGDeliveryContract
		maxTokens = iqSVGMaxTokens
	default:
		res.Verdict = IQVerdictError
		res.ErrorMessage = "unsupported question kind: " + q.Kind
		return res
	}
	res.Prompt = prompt

	text, latency, err := s.callAccount(ctx, accountID, modelID, prompt, maxTokens)
	res.ResponseText = text
	res.LatencyMs = latency
	if err != nil {
		res.Verdict = IQVerdictError
		res.ErrorMessage = err.Error()
		return res
	}

	switch q.Kind {
	case IQKindCandy:
		if IQCandyAnswerCorrect(text) {
			res.Verdict = IQVerdictPassed
			res.Score = iqFloatPtr(100)
		} else {
			res.Verdict = IQVerdictFailed
			res.Score = iqFloatPtr(0)
			res.Detail["reason"] = "answer_mismatch: expected 21"
		}
	case IQKindRandomReasoning:
		ok, reason := IQRandomAnswerCorrect(random.Family, random.Reference, text)
		if ok {
			res.Verdict = IQVerdictPassed
			res.Score = iqFloatPtr(100)
		} else {
			res.Verdict = IQVerdictFailed
			res.Score = iqFloatPtr(0)
			res.Detail["reason"] = reason
		}
	case IQKindSVGDraw:
		s.evaluateSVG(ctx, q, modelID, text, res)
	}
	return res
}

func iqFloatPtr(v float64) *float64 { return &v }

// callAccount 通过账号测试链路发送 prompt 并收集完整文本输出。
func (s *IQTestService) callAccount(ctx context.Context, accountID int64, modelID, prompt string, maxTokens int) (string, int64, error) {
	if s.accountTestSvc == nil {
		return "", 0, errors.New("account test service unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, iqSampleTimeout)
	defer cancel()
	ctx = withIQPromptOverride(ctx, prompt, maxTokens)

	start := time.Now()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = (&http.Request{}).WithContext(ctx)
	ginCtx.Set(accountTestBackgroundKey, true)
	testErr := s.accountTestSvc.TestAccountConnection(ginCtx, accountID, modelID, prompt, AccountTestModeDefault)
	latency := time.Since(start).Milliseconds()
	text, errMsg := parseTestSSEOutput(w.Body.String())
	if errMsg == "" && testErr != nil {
		errMsg = testErr.Error()
	}
	if errMsg != "" {
		return text, latency, errors.New(errMsg)
	}
	if strings.TrimSpace(text) == "" {
		return text, latency, errors.New("empty response")
	}
	return text, latency, nil
}

// evaluateSVG 画图题评估：硬门槛 → 参考分布比对（模型级优先，回落到题目级）→ 可选分类器。
func (s *IQTestService) evaluateSVG(ctx context.Context, q *IQQuestion, modelID, text string, res *IQTestResult) {
	a := AnalyzeIQSVG(text, q.Keywords)
	res.Detail["features"] = a.Features
	res.Detail["anim_source"] = a.AnimSource
	if len(a.HardFails) > 0 {
		res.Verdict = IQVerdictFailed
		res.Score = iqFloatPtr(0)
		res.Detail["hard_fails"] = a.HardFails
		return
	}
	set, classifier, err := s.referenceSet(ctx, q, modelID)
	if err != nil {
		res.Verdict = IQVerdictInconclusive
		res.Detail["reason"] = "reference_load_failed: " + err.Error()
		return
	}
	if set == nil {
		res.Verdict = IQVerdictInconclusive
		res.Detail["reason"] = "insufficient_references"
		return
	}
	score := set.Score(a)
	if classifier != nil {
		if p, ok := classifier.Predict(a.Features); ok {
			score.ClassifierProb = &p
		}
	}
	res.Detail["svg_score"] = score
	res.Verdict = score.Verdict()
	res.Score = iqFloatPtr(score.Score100())
}

// referenceSet 返回（带缓存的）参考集：同模型样本足够时用模型级，否则用题目级。
func (s *IQTestService) referenceSet(ctx context.Context, q *IQQuestion, modelID string) (*IQSVGReferenceSet, *IQLogisticModel, error) {
	key := q.Slug + "\x00" + modelID
	s.refMu.Lock()
	if e, ok := s.refCache[key]; ok && time.Since(e.builtAt) < 10*time.Minute {
		s.refMu.Unlock()
		return e.set, e.classifier, nil
	}
	s.refMu.Unlock()

	samples, err := s.repo.ListSamples(ctx, q.Slug, true)
	if err != nil {
		return nil, nil, err
	}
	var modelGood, allGood []*IQSVGAnalysis
	for _, smp := range samples {
		if smp.Label != "good" {
			continue
		}
		an := AnalyzeIQSVG(smp.SVG, q.Keywords)
		allGood = append(allGood, an)
		if modelID != "" && strings.EqualFold(smp.ModelID, modelID) {
			modelGood = append(modelGood, an)
		}
	}
	set := BuildIQSVGReferenceSet(modelGood, "model")
	if set == nil {
		set = BuildIQSVGReferenceSet(allGood, "question")
	}
	var classifier *IQLogisticModel
	if info, err := s.repo.GetClassifier(ctx, q.Slug); err == nil && info != nil {
		classifier = info.Model
	}
	s.refMu.Lock()
	s.refCache[key] = &iqRefCacheEntry{set: set, classifier: classifier, builtAt: time.Now()}
	s.refMu.Unlock()
	return set, classifier, nil
}

func (s *IQTestService) invalidateReferenceCache(slug string) {
	s.refMu.Lock()
	defer s.refMu.Unlock()
	for k := range s.refCache {
		if strings.HasPrefix(k, slug+"\x00") {
			delete(s.refCache, k)
		}
	}
}

// ---------------- 参考样本与分类器 ----------------

// AddSample 上传参考样本（自动从 HTML/Markdown 中提取 SVG 并校验）。
func (s *IQTestService) AddSample(ctx context.Context, smp *IQReferenceSample) (*IQReferenceSample, error) {
	q, err := s.repo.GetQuestion(ctx, smp.QuestionSlug)
	if err != nil || q == nil || q.Kind != IQKindSVGDraw {
		return nil, ErrIQQuestionNotFound
	}
	if smp.Label != "bad" {
		smp.Label = "good"
	}
	a := AnalyzeIQSVG(smp.SVG, q.Keywords)
	if a.SVG == "" {
		return nil, fmt.Errorf("%w: no <svg> found", ErrIQInvalidInput)
	}
	if smp.Label == "good" && len(a.HardFails) > 0 {
		return nil, fmt.Errorf("%w: sample fails hard checks: %s", ErrIQInvalidInput, strings.Join(a.HardFails, ","))
	}
	smp.SVG = a.SVG
	smp.Features = a.Features
	smp.ModelID = strings.TrimSpace(smp.ModelID)
	if smp.Source == "" {
		smp.Source = "upload"
	}
	saved, err := s.repo.CreateSample(ctx, smp)
	if err == nil {
		s.invalidateReferenceCache(smp.QuestionSlug)
	}
	return saved, err
}

func (s *IQTestService) ListSamples(ctx context.Context, slug string) ([]*IQReferenceSample, error) {
	return s.repo.ListSamples(ctx, slug, false)
}

func (s *IQTestService) DeleteSample(ctx context.Context, slug string, id int64) error {
	err := s.repo.DeleteSample(ctx, id)
	if err == nil {
		s.invalidateReferenceCache(slug)
	}
	return err
}

func (s *IQTestService) SampleStats(ctx context.Context) ([]IQSampleStat, error) {
	return s.repo.SampleStats(ctx)
}

// LabelResult 人工标注一条画图题结果（good/bad），并作为参考样本入库。
func (s *IQTestService) LabelResult(ctx context.Context, resultID int64, label string) error {
	if label != "good" && label != "bad" {
		return ErrIQInvalidInput
	}
	r, err := s.repo.GetResult(ctx, resultID)
	if err != nil {
		return err
	}
	if err := s.repo.SetResultLabel(ctx, resultID, label); err != nil {
		return err
	}
	q, err := s.repo.GetQuestion(ctx, r.QuestionSlug)
	if err != nil || q.Kind != IQKindSVGDraw {
		return nil // 答案题只记录标注
	}
	_, err = s.AddSample(ctx, &IQReferenceSample{
		QuestionSlug: r.QuestionSlug,
		ModelID:      r.ModelID,
		Label:        label,
		Source:       "result",
		Note:         fmt.Sprintf("result #%d account #%d", r.ID, r.AccountID),
		SVG:          r.ResponseText,
	})
	return err
}

// RetrainClassifier 用该题的 good/bad 参考样本训练逻辑回归。
func (s *IQTestService) RetrainClassifier(ctx context.Context, slug string) (*IQClassifierInfo, error) {
	q, err := s.repo.GetQuestion(ctx, slug)
	if err != nil || q.Kind != IQKindSVGDraw {
		return nil, ErrIQQuestionNotFound
	}
	samples, err := s.repo.ListSamples(ctx, slug, true)
	if err != nil {
		return nil, err
	}
	var good, bad [][]float64
	for _, smp := range samples {
		a := AnalyzeIQSVG(smp.SVG, q.Keywords)
		if smp.Label == "bad" {
			bad = append(bad, a.Vector())
		} else if len(a.HardFails) == 0 {
			good = append(good, a.Vector())
		}
	}
	model, acc := TrainIQLogisticModel(good, bad)
	if model == nil {
		return nil, fmt.Errorf("%w: need at least %d good and %d bad samples (have %d / %d)", ErrIQInvalidInput, IQMinClassifierSamples, IQMinClassifierSamples, len(good), len(bad))
	}
	info := &IQClassifierInfo{QuestionSlug: slug, Model: model, NGood: len(good), NBad: len(bad), Accuracy: acc, TrainedAt: time.Now()}
	if err := s.repo.SaveClassifier(ctx, info); err != nil {
		return nil, err
	}
	s.invalidateReferenceCache(slug)
	return info, nil
}

func (s *IQTestService) GetClassifier(ctx context.Context, slug string) (*IQClassifierInfo, error) {
	return s.repo.GetClassifier(ctx, slug)
}

// ---------------- 结果 ----------------

func (s *IQTestService) ListResults(ctx context.Context, f IQResultFilter) ([]*IQTestResult, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	return s.repo.ListResults(ctx, f)
}

// ---------------- 计划 ----------------

func normalizeIQPlan(p *IQTestPlan) error {
	p.ParallelCount = clampIQParallel(p.ParallelCount)
	if p.FailThreshold < 1 {
		p.FailThreshold = 2
	}
	if p.MaxResults < 1 || p.MaxResults > 500 {
		p.MaxResults = 50
	}
	switch p.FailAction {
	case IQActionObserveOnly, IQActionHealthPenalty, IQActionDisableScheduling:
	case "":
		p.FailAction = IQActionObserveOnly
	default:
		return fmt.Errorf("%w: unknown fail_action", ErrIQInvalidInput)
	}
	p.CronExpression = strings.TrimSpace(p.CronExpression)
	if p.CronExpression == "" {
		p.CronExpression = "0 */6 * * *"
	}
	next, err := computeNextRun(p.CronExpression, time.Now())
	if err != nil {
		return fmt.Errorf("%w: invalid cron expression", ErrIQInvalidInput)
	}
	p.NextRunAt = &next
	return nil
}

func (s *IQTestService) CreatePlan(ctx context.Context, p *IQTestPlan) (*IQTestPlan, error) {
	if p.AccountID <= 0 {
		return nil, ErrIQInvalidInput
	}
	if _, err := s.repo.GetQuestion(ctx, p.QuestionSlug); err != nil {
		return nil, ErrIQQuestionNotFound
	}
	if err := normalizeIQPlan(p); err != nil {
		return nil, err
	}
	return s.repo.CreatePlan(ctx, p)
}

func (s *IQTestService) UpdatePlan(ctx context.Context, p *IQTestPlan) (*IQTestPlan, error) {
	existing, err := s.repo.GetPlan(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetQuestion(ctx, p.QuestionSlug); err != nil {
		return nil, ErrIQQuestionNotFound
	}
	if err := normalizeIQPlan(p); err != nil {
		return nil, err
	}
	p.AccountID = existing.AccountID
	// 处置方式变更或停用时先撤销已施加的处置，避免账号被遗留在惩罚状态
	if existing.ActionApplied && (existing.FailAction != p.FailAction || !p.Enabled) {
		s.revertAction(ctx, existing)
		existing.ActionApplied = false
		existing.ConsecutiveFails = 0
		_ = s.repo.UpdatePlanAfterRun(ctx, existing)
	}
	return s.repo.UpdatePlan(ctx, p)
}

func (s *IQTestService) DeletePlan(ctx context.Context, id int64) error {
	if p, err := s.repo.GetPlan(ctx, id); err == nil && p.ActionApplied {
		s.revertAction(ctx, p)
	}
	return s.repo.DeletePlan(ctx, id)
}

func (s *IQTestService) ListPlansByAccount(ctx context.Context, accountID int64) ([]*IQTestPlan, error) {
	return s.repo.ListPlansByAccount(ctx, accountID)
}

// RunPlanNow 立即执行一次计划（含处置逻辑）。
func (s *IQTestService) RunPlanNow(ctx context.Context, id int64) (*IQRoundResult, error) {
	p, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.runPlan(ctx, p, false)
}

func (s *IQTestService) runPlan(ctx context.Context, p *IQTestPlan, advanceSchedule bool) (*IQRoundResult, error) {
	planID := p.ID
	round, err := s.RunRound(ctx, IQRunRequest{AccountID: p.AccountID, ModelID: p.ModelID, QuestionSlug: p.QuestionSlug, ParallelCount: p.ParallelCount}, &planID)
	now := time.Now()
	p.LastRunAt = &now
	if advanceSchedule {
		if next, cerr := computeNextRun(p.CronExpression, now); cerr == nil {
			p.NextRunAt = &next
		}
	}
	if err != nil {
		p.LastVerdict = IQVerdictError
		_ = s.repo.UpdatePlanAfterRun(ctx, p)
		return nil, err
	}
	p.LastVerdict = round.Verdict
	s.applyRoundOutcome(ctx, p, round.Verdict)
	if err := s.repo.UpdatePlanAfterRun(ctx, p); err != nil {
		logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d update after run failed: %v", p.ID, err)
	}
	_ = s.repo.PruneResults(ctx, p.ID, p.MaxResults)
	return round, nil
}

// applyRoundOutcome 处置状态机：连续失败达到阈值施加处置；一轮通过即撤销。
func (s *IQTestService) applyRoundOutcome(ctx context.Context, p *IQTestPlan, verdict string) {
	switch verdict {
	case IQVerdictFailed:
		p.ConsecutiveFails++
		if !p.ActionApplied && p.ConsecutiveFails >= p.FailThreshold && p.FailAction != IQActionObserveOnly {
			if s.applyAction(ctx, p) {
				p.ActionApplied = true
			}
		}
	case IQVerdictPassed:
		p.ConsecutiveFails = 0
		if p.ActionApplied {
			s.revertAction(ctx, p)
			p.ActionApplied = false
		}
	}
}

func (s *IQTestService) applyAction(ctx context.Context, p *IQTestPlan) bool {
	switch p.FailAction {
	case IQActionHealthPenalty:
		s.health.SetPenalty(p.AccountID, iqHealthPenaltyValue, time.Time{}, fmt.Sprintf("iq_test:%s", p.QuestionSlug))
		logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d account=%d health penalty applied", p.ID, p.AccountID)
		return true
	case IQActionDisableScheduling:
		if s.accountRepo == nil {
			return false
		}
		acc, err := s.accountRepo.GetByID(ctx, p.AccountID)
		if err != nil || acc == nil || !acc.Schedulable {
			// 账号已被人工关闭调度：不接管，避免之后误恢复
			return false
		}
		if err := s.accountRepo.SetSchedulable(ctx, p.AccountID, false); err != nil {
			logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d disable scheduling failed: %v", p.ID, err)
			return false
		}
		logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d account=%d scheduling disabled", p.ID, p.AccountID)
		return true
	}
	return false
}

func (s *IQTestService) revertAction(ctx context.Context, p *IQTestPlan) {
	switch p.FailAction {
	case IQActionHealthPenalty:
		s.health.ClearPenalty(p.AccountID)
	case IQActionDisableScheduling:
		if s.accountRepo != nil {
			if err := s.accountRepo.SetSchedulable(ctx, p.AccountID, true); err != nil {
				logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d restore scheduling failed: %v", p.ID, err)
				return
			}
		}
	}
	logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d account=%d action reverted", p.ID, p.AccountID)
}

// ---------------- 定时执行 ----------------

// Start 启动每分钟扫描的定时器，并同步健康惩罚（多实例下各实例每分钟从库里恢复惩罚状态）。
func (s *IQTestService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.SeedBuiltinQuestions(ctx); err != nil {
				logger.LegacyPrintf("service.iq_test", "[IQTest] seed builtin questions failed: %v", err)
			}
			s.syncPenalties(ctx)
		}()
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}
		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		if _, err := c.AddFunc("* * * * *", s.tick); err != nil {
			logger.LegacyPrintf("service.iq_test", "[IQTest] runner not started: %v", err)
			return
		}
		s.cron = c
		c.Start()
	})
}

// Stop 停止定时器。
func (s *IQTestService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	})
}

func (s *IQTestService) syncPenalties(ctx context.Context) {
	plans, err := s.repo.ListPenaltyPlans(ctx)
	if err != nil {
		return
	}
	for _, p := range plans {
		s.health.SetPenalty(p.AccountID, iqHealthPenaltyValue, time.Time{}, fmt.Sprintf("iq_test:%s", p.QuestionSlug))
	}
}

func (s *IQTestService) tick() {
	time.Sleep(20 * time.Second) // 与定时连通性测试错开
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	s.syncPenalties(ctx)

	plans, err := s.repo.ListDuePlans(ctx, time.Now())
	if err != nil || len(plans) == 0 {
		return
	}
	// 先推进 next_run_at，避免多实例 / 长任务重复执行
	for _, p := range plans {
		if next, err := computeNextRun(p.CronExpression, time.Now()); err == nil {
			p.NextRunAt = &next
			_ = s.repo.UpdatePlanAfterRun(ctx, p)
		}
	}
	sem := make(chan struct{}, iqRunnerWorkers)
	var wg sync.WaitGroup
	for _, p := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *IQTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d panic: %v", p.ID, r)
				}
			}()
			if _, err := s.runPlan(ctx, p, false); err != nil {
				logger.LegacyPrintf("service.iq_test", "[IQTest] plan=%d run failed: %v", p.ID, err)
			}
		}(p)
	}
	wg.Wait()
}
