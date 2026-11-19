package service

import (
	"math"
	"sort"
)

// 画图题本地评估：基于参考样本的单类评分（鲁棒 z 距离 + TF-IDF 风格相似度）
// 与有负样本后的逻辑回归二分类。阈值由参考集留一法(LOO)自动校准，不需要手调魔数。

const (
	// IQMinReferenceSamples 参考样本少于该值时不做分布比对（结果为 inconclusive）。
	IQMinReferenceSamples = 8
	// IQRecommendedReferenceSamples 建议的参考样本数量（前端提示用）。
	IQRecommendedReferenceSamples = 20
	// IQMinClassifierSamples 正负样本各达到该数量才训练逻辑回归。
	IQMinClassifierSamples = 10

	iqZClip         = 6.0
	iqPassQuantile  = 0.95
	iqPassMargin    = 1.15
	iqMinPassThresh = 0.6
	iqStyleTopK     = 3
)

// IQSVGReferenceSet 是一组已向量化的参考样本（同一题目，可选同一模型）。
type IQSVGReferenceSet struct {
	Scope          string // model | question
	N              int
	median         []float64
	scale          []float64
	tfidf          []map[string]float64
	idf            map[string]float64
	PassThreshold  float64
	FailThreshold  float64
	StyleThreshold float64
}

// IQSVGScore 是一次画图题评估结果明细。
type IQSVGScore struct {
	Distance       float64            `json:"distance"`
	PassThreshold  float64            `json:"pass_threshold"`
	FailThreshold  float64            `json:"fail_threshold"`
	Style          float64            `json:"style_similarity"`
	StyleThreshold float64            `json:"style_threshold"`
	ReferenceScope string             `json:"reference_scope"`
	ReferenceCount int                `json:"reference_count"`
	WorstFeatures  map[string]float64 `json:"worst_features,omitempty"` // 偏离最大的特征及其 z 值
	ClassifierProb *float64           `json:"classifier_prob,omitempty"`
	Warnings       []string           `json:"warnings,omitempty"`
}

// BuildIQSVGReferenceSet 用参考样本分析结果构建参考集；样本不足返回 nil。
func BuildIQSVGReferenceSet(samples []*IQSVGAnalysis, scope string) *IQSVGReferenceSet {
	usable := make([]*IQSVGAnalysis, 0, len(samples))
	for _, s := range samples {
		if s != nil && len(s.HardFails) == 0 {
			usable = append(usable, s)
		}
	}
	if len(usable) < IQMinReferenceSamples {
		return nil
	}
	set := &IQSVGReferenceSet{Scope: scope, N: len(usable)}
	vectors := make([][]float64, len(usable))
	for i, s := range usable {
		vectors[i] = s.Vector()
	}
	set.median, set.scale = iqRobustStats(vectors)

	// TF-IDF
	df := map[string]float64{}
	for _, s := range usable {
		for tok := range s.Tokens {
			df[tok]++
		}
	}
	n := float64(len(usable))
	set.idf = make(map[string]float64, len(df))
	for tok, c := range df {
		set.idf[tok] = math.Log((1+n)/(1+c)) + 1
	}
	set.tfidf = make([]map[string]float64, len(usable))
	for i, s := range usable {
		set.tfidf[i] = iqTFIDF(s.Tokens, set.idf)
	}

	// 留一法校准阈值：每个参考样本相对「其余样本」的距离 / 风格相似度分布
	looDist := make([]float64, len(usable))
	looStyle := make([]float64, len(usable))
	for i := range usable {
		rest := make([][]float64, 0, len(vectors)-1)
		restTF := make([]map[string]float64, 0, len(vectors)-1)
		for j := range vectors {
			if j != i {
				rest = append(rest, vectors[j])
				restTF = append(restTF, set.tfidf[j])
			}
		}
		med, sc := iqRobustStats(rest)
		looDist[i], _ = iqRobustDistance(vectors[i], med, sc)
		looStyle[i] = iqKNNSimilarity(set.tfidf[i], restTF)
	}
	set.PassThreshold = math.Max(iqQuantile(looDist, iqPassQuantile)*iqPassMargin, iqMinPassThresh)
	set.FailThreshold = math.Max(set.PassThreshold*1.8, set.PassThreshold+1.0)
	set.StyleThreshold = iqQuantile(looStyle, 0.05) * 0.85
	return set
}

// Score 计算候选 SVG 相对参考集的距离与风格相似度。
func (set *IQSVGReferenceSet) Score(a *IQSVGAnalysis) *IQSVGScore {
	dist, zs := iqRobustDistance(a.Vector(), set.median, set.scale)
	tf := iqTFIDF(a.Tokens, set.idf)
	style := iqKNNSimilarity(tf, set.tfidf)
	res := &IQSVGScore{
		Distance:       dist,
		PassThreshold:  set.PassThreshold,
		FailThreshold:  set.FailThreshold,
		Style:          style,
		StyleThreshold: set.StyleThreshold,
		ReferenceScope: set.Scope,
		ReferenceCount: set.N,
		WorstFeatures:  map[string]float64{},
	}
	type fz struct {
		name string
		z    float64
	}
	list := make([]fz, 0, len(zs))
	for i, z := range zs {
		if z != 0 {
			list = append(list, fz{IQSVGFeatureNames[i], z})
		}
	}
	sort.Slice(list, func(i, j int) bool { return math.Abs(list[i].z) > math.Abs(list[j].z) })
	for i := 0; i < len(list) && i < 5; i++ {
		res.WorstFeatures[list[i].name] = math.Round(list[i].z*100) / 100
	}
	if style < set.StyleThreshold {
		res.Warnings = append(res.Warnings, "style_mismatch")
	}
	return res
}

// Verdict 依据距离阈值给出 passed / failed / inconclusive。
func (s *IQSVGScore) Verdict() string {
	if s.ClassifierProb != nil {
		p := *s.ClassifierProb
		switch {
		case p >= 0.6 && s.Distance <= s.FailThreshold:
			return "passed"
		case p < 0.3:
			return "failed"
		}
	}
	switch {
	case s.Distance <= s.PassThreshold:
		return "passed"
	case s.Distance > s.FailThreshold:
		return "failed"
	default:
		return "inconclusive"
	}
}

// Score100 把距离映射到 0~100 的展示分（阈值处约 60 分）。
func (s *IQSVGScore) Score100() float64 {
	if s.ClassifierProb != nil {
		return math.Round(*s.ClassifierProb * 100)
	}
	if s.PassThreshold <= 0 {
		return 0
	}
	v := 100 * math.Exp(-0.51*s.Distance/s.PassThreshold)
	return math.Round(v)
}

func iqRobustStats(vectors [][]float64) (median, scale []float64) {
	if len(vectors) == 0 {
		return nil, nil
	}
	dims := len(vectors[0])
	median = make([]float64, dims)
	scale = make([]float64, dims)
	col := make([]float64, len(vectors))
	for d := 0; d < dims; d++ {
		for i, v := range vectors {
			col[i] = v[d]
		}
		m := iqQuantile(col, 0.5)
		dev := make([]float64, len(col))
		for i, x := range col {
			dev[i] = math.Abs(x - m)
		}
		mad := iqQuantile(dev, 0.5) * 1.4826
		// 下限：避免参考集在某一维完全一致时 z 爆炸
		floor := math.Max(0.1*math.Abs(m), 0.05)
		if mad < floor {
			mad = floor
		}
		median[d] = m
		scale[d] = mad
	}
	return median, scale
}

// iqRobustDistance 返回平均截断 |z|，复杂度特征只计「低于中位数」的偏离。
func iqRobustDistance(x, median, scale []float64) (float64, []float64) {
	zs := make([]float64, len(x))
	if len(x) == 0 || len(median) != len(x) {
		return 0, zs
	}
	sum := 0.0
	for i := range x {
		z := (x[i] - median[i]) / scale[i]
		name := IQSVGFeatureNames[i]
		if iqSVGComplexityFeatures[name] && z > 0 {
			z = 0
		}
		if name == "offcanvas_ratio" && z < 0 {
			z = 0 // 画布外元素少是好事
		}
		if z > iqZClip {
			z = iqZClip
		} else if z < -iqZClip {
			z = -iqZClip
		}
		zs[i] = z
		sum += math.Abs(z)
	}
	return sum / float64(len(x)), zs
}

func iqTFIDF(tokens map[string]float64, idf map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(tokens))
	norm := 0.0
	for tok, c := range tokens {
		w, ok := idf[tok]
		if !ok {
			w = math.Log(1+float64(len(idf))) + 1 // 参考集中没见过的 token
		}
		v := (1 + math.Log(c)) * w
		out[tok] = v
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for tok := range out {
			out[tok] /= norm
		}
	}
	return out
}

func iqCosine(a, b map[string]float64) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	dot := 0.0
	for k, v := range a {
		dot += v * b[k]
	}
	return dot
}

func iqKNNSimilarity(x map[string]float64, refs []map[string]float64) float64 {
	if len(refs) == 0 {
		return 0
	}
	sims := make([]float64, len(refs))
	for i, r := range refs {
		sims[i] = iqCosine(x, r)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sims)))
	k := iqStyleTopK
	if k > len(sims) {
		k = len(sims)
	}
	sum := 0.0
	for i := 0; i < k; i++ {
		sum += sims[i]
	}
	return sum / float64(k)
}

func iqQuantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo]
	}
	frac := pos - float64(lo)
	return s[lo]*(1-frac) + s[hi]*frac
}

// ---------------- 逻辑回归（有负样本时启用） ----------------

// IQLogisticModel 是按题目训练的二分类器（特征标准化后的逻辑回归）。
type IQLogisticModel struct {
	FeatureNames []string  `json:"feature_names"`
	Mean         []float64 `json:"mean"`
	Std          []float64 `json:"std"`
	Weights      []float64 `json:"weights"`
	Bias         float64   `json:"bias"`
}

// Predict 返回「合格」概率；特征版本不一致时返回 ok=false。
func (m *IQLogisticModel) Predict(features map[string]float64) (float64, bool) {
	if m == nil || len(m.FeatureNames) != len(IQSVGFeatureNames) {
		return 0, false
	}
	for i, name := range IQSVGFeatureNames {
		if m.FeatureNames[i] != name {
			return 0, false
		}
	}
	x := iqFeatureVector(features)
	z := m.Bias
	for i := range x {
		z += m.Weights[i] * (x[i] - m.Mean[i]) / m.Std[i]
	}
	return 1 / (1 + math.Exp(-z)), true
}

// TrainIQLogisticModel 训练逻辑回归；返回模型与 5 折交叉验证准确率。
func TrainIQLogisticModel(good, bad [][]float64) (*IQLogisticModel, float64) {
	if len(good) < IQMinClassifierSamples || len(bad) < IQMinClassifierSamples {
		return nil, 0
	}
	xs := append(append([][]float64{}, good...), bad...)
	ys := make([]float64, len(xs))
	for i := range good {
		ys[i] = 1
	}
	model := fitIQLogistic(xs, ys)

	// 5 折交叉验证（按下标交错分折）
	correct, total := 0, 0
	const folds = 5
	for f := 0; f < folds; f++ {
		var trX, teX [][]float64
		var trY, teY []float64
		for i := range xs {
			if i%folds == f {
				teX, teY = append(teX, xs[i]), append(teY, ys[i])
			} else {
				trX, trY = append(trX, xs[i]), append(trY, ys[i])
			}
		}
		if len(teX) == 0 || len(trX) == 0 {
			continue
		}
		m := fitIQLogistic(trX, trY)
		for i, x := range teX {
			p := m.predictVec(x)
			if (p >= 0.5) == (teY[i] == 1) {
				correct++
			}
			total++
		}
	}
	acc := 0.0
	if total > 0 {
		acc = float64(correct) / float64(total)
	}
	return model, acc
}

func (m *IQLogisticModel) predictVec(x []float64) float64 {
	z := m.Bias
	for i := range x {
		z += m.Weights[i] * (x[i] - m.Mean[i]) / m.Std[i]
	}
	return 1 / (1 + math.Exp(-z))
}

func fitIQLogistic(xs [][]float64, ys []float64) *IQLogisticModel {
	dims := len(IQSVGFeatureNames)
	m := &IQLogisticModel{
		FeatureNames: append([]string(nil), IQSVGFeatureNames...),
		Mean:         make([]float64, dims),
		Std:          make([]float64, dims),
		Weights:      make([]float64, dims),
	}
	n := float64(len(xs))
	for _, x := range xs {
		for d := 0; d < dims; d++ {
			m.Mean[d] += x[d] / n
		}
	}
	for _, x := range xs {
		for d := 0; d < dims; d++ {
			diff := x[d] - m.Mean[d]
			m.Std[d] += diff * diff / n
		}
	}
	for d := range m.Std {
		m.Std[d] = math.Sqrt(m.Std[d])
		if m.Std[d] < 1e-6 {
			m.Std[d] = 1
		}
	}
	// 类别不平衡加权
	pos := 0.0
	for _, y := range ys {
		pos += y
	}
	wPos, wNeg := n/(2*math.Max(pos, 1)), n/(2*math.Max(n-pos, 1))
	const (
		lr     = 0.1
		l2     = 0.01
		epochs = 600
	)
	grad := make([]float64, dims)
	for e := 0; e < epochs; e++ {
		for d := range grad {
			grad[d] = 0
		}
		gb := 0.0
		for i, x := range xs {
			p := m.predictVec(x)
			w := wNeg
			if ys[i] == 1 {
				w = wPos
			}
			diff := (p - ys[i]) * w
			for d := 0; d < dims; d++ {
				grad[d] += diff * (x[d] - m.Mean[d]) / m.Std[d]
			}
			gb += diff
		}
		for d := 0; d < dims; d++ {
			m.Weights[d] -= lr * (grad[d]/n + l2*m.Weights[d])
		}
		m.Bias -= lr * gb / n
	}
	return m
}
