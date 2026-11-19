//go:build unit

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	mathrand "math/rand/v2"
	"strings"
	"testing"
)

func TestIQCandyAnswerCorrect(t *testing.T) {
	ok := []string{"21", "答案是21", "最少取出21个糖果", "**21**", "２１", "二十一", "<think>算一下...29?</think>21", "The answer is 21."}
	bad := []string{"29", "2 1", "21或22", "答案为 20", "", "至少需要取出 210 个"}
	for _, s := range ok {
		if !IQCandyAnswerCorrect(s) {
			t.Errorf("expected correct: %q", s)
		}
	}
	for _, s := range bad {
		if IQCandyAnswerCorrect(s) {
			t.Errorf("expected wrong: %q", s)
		}
	}
}

// 暴力枚举所有 R/U 序列验证 DP 求解器。
func bruteIQPath(p iqPathProblem) iqPathAnswer {
	steps := p.Width + p.Height
	var ans iqPathAnswer
	for mask := 0; mask < 1<<steps; mask++ {
		var sb strings.Builder
		rs := 0
		for i := steps - 1; i >= 0; i-- {
			if mask>>i&1 == 0 {
				sb.WriteByte('R')
				rs++
			} else {
				sb.WriteByte('U')
			}
		}
		if rs != p.Width {
			continue
		}
		seq := sb.String()
		if strings.Contains(seq, "RRR") {
			continue
		}
		x, y, turns, hits, okPath := 0, 0, 0, 0, true
		for i, c := range seq {
			if c == 'R' {
				x++
			} else {
				y++
			}
			if i > 0 && seq[i] != seq[i-1] {
				turns++
			}
			for _, b := range p.Blocked {
				if b.X == x && b.Y == y {
					okPath = false
				}
			}
			for _, cp := range p.Checkpoints {
				if cp.X == x && cp.Y == y {
					hits++
				}
			}
		}
		if okPath && turns == p.Turns && hits == 1 {
			if ans.Count == 0 || seq < ans.First {
				ans.First = seq
			}
			ans.Count++
		}
	}
	return ans
}

func TestIQPathSolverMatchesBruteForce(t *testing.T) {
	rng := mathrand.New(mathrand.NewPCG(1, 2))
	for i := 0; i < 30; i++ {
		p := iqPathProblem{Width: 3 + rng.IntN(3), Height: 3 + rng.IntN(3), Turns: 2 + rng.IntN(4)}
		var pts []iqGridPoint
		for x := 0; x <= p.Width; x++ {
			for y := 0; y <= p.Height; y++ {
				if (x != 0 || y != 0) && (x != p.Width || y != p.Height) {
					pts = append(pts, iqGridPoint{x, y})
				}
			}
		}
		order := rng.Perm(len(pts))
		for j := range p.Blocked {
			p.Blocked[j] = pts[order[j]]
		}
		for j := range p.Checkpoints {
			p.Checkpoints[j] = pts[order[j+3]]
		}
		got, want := solveIQPath(p), bruteIQPath(p)
		if got.Count != want.Count || (want.Count > 0 && got.First != want.First) {
			t.Fatalf("problem %+v: got %+v want %+v", p, got, want)
		}
	}
}

func TestIQRandomQuestionReferenceAccepted(t *testing.T) {
	for fam := 0; fam < 2; fam++ {
		for i := 0; i < 10; i++ {
			seed := bytes.Repeat([]byte{byte(i*7 + fam + 1)}, 16)
			q, err := generateIQRandomQuestion(bytes.NewReader(seed), fam)
			if err != nil {
				t.Fatal(err)
			}
			if ok, reason := IQRandomAnswerCorrect(q.Family, q.Reference, "```json\n"+q.Reference+"\n```"); !ok {
				t.Fatalf("reference rejected (%s): %s", reason, q.Reference)
			}
			if ok, _ := IQRandomAnswerCorrect(q.Family, q.Reference, `{"count":-1,"first":"R","penalty":-1,"assignments":[]}`); ok {
				t.Fatalf("wrong answer accepted for %s", q.Family)
			}
		}
	}
}

func TestIQXORAssignmentsOrderInsensitive(t *testing.T) {
	ref := `{"penalty":3,"assignments":["010101","110000"]}`
	if ok, r := IQRandomAnswerCorrect("weighted_xor", ref, `{"penalty":3,"assignments":["110000","010101"]}`); !ok {
		t.Fatalf("should accept reordered assignments: %s", r)
	}
}

// ---------------- SVG ----------------

func richIQSVG(rng *mathrand.Rand) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600">`)
	b.WriteString(`<defs><linearGradient id="sky"><stop offset="0" stop-color="#87ceeb"/><stop offset="1" stop-color="#fff"/></linearGradient></defs>`)
	b.WriteString(`<style>@keyframes spin{to{transform:rotate(360deg)}} @keyframes pedal{50%{transform:rotate(20deg)}} @keyframes road{to{transform:translateX(-800px)}} .wheel{animation:spin 1s linear infinite}</style>`)
	b.WriteString(`<rect width="800" height="600" fill="url(#sky)"/>`)
	b.WriteString(`<!-- road and signs -->`)
	b.WriteString(fmt.Sprintf(`<g id="road"><rect x="0" y="480" width="800" height="120" fill="#555"/><rect x="%d" y="400" width="10" height="80" fill="#8b4513"/></g>`, 100+rng.IntN(300)))
	for w := 0; w < 2; w++ {
		cx := 280 + w*240 + rng.IntN(20)
		b.WriteString(fmt.Sprintf(`<g class="wheel" id="wheel%d"><circle cx="%d" cy="420" r="%d" fill="none" stroke="#222" stroke-width="8"/>`, w, cx, 70+rng.IntN(10)))
		for s := 0; s < 6+rng.IntN(4); s++ {
			b.WriteString(fmt.Sprintf(`<line x1="%d" y1="420" x2="%d" y2="%d" stroke="#999"/>`, cx, cx+rng.IntN(60)-30, 360+rng.IntN(120)))
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`<g id="frame"><path d="M280 420 L400 300 L520 420 L400 420 Z" fill="none" stroke="#c0392b" stroke-width="10"/><path d="M400 300 L380 260 M500 300 L520 250" stroke="#333"/></g>`)
	b.WriteString(`<g id="pedal" style="animation:pedal 1s infinite"><circle cx="400" cy="420" r="18" fill="#444"/></g>`)
	b.WriteString(fmt.Sprintf(`<g id="pelican"><ellipse cx="%d" cy="230" rx="90" ry="60" fill="#f5f5f5" stroke="#333"/><path id="beak" d="M470 180 Q600 190 640 220 Q560 240 470 205 Z" fill="#f39c12"/><path id="pouch" d="M480 205 Q560 260 620 225" fill="#e67e22"/><circle id="eye" cx="455" cy="170" r="7" fill="#000"/><path id="wing" d="M330 220 Q300 260 360 280" fill="#ddd"/><path id="leg" d="M400 280 L410 360 L400 420" stroke="#f39c12" stroke-width="6" fill="none"/></g>`, 380+rng.IntN(30)))
	b.WriteString(`<animateTransform href="#road" attributeName="transform" type="translate" from="0 0" to="-200 0" dur="3s" repeatCount="indefinite"/>`)
	b.WriteString(`</svg>`)
	return b.String()
}

func simpleIQSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600"><style>@keyframes s{to{transform:rotate(360deg)}}</style>` +
		`<circle cx="300" cy="400" r="50" fill="none" stroke="black"/><circle cx="500" cy="400" r="50" fill="none" stroke="black" style="animation:s 1s infinite"/>` +
		`<line x1="300" y1="400" x2="500" y2="400" stroke="black"/><ellipse cx="400" cy="300" rx="40" ry="30" fill="white"/><rect x="0" y="450" width="800" height="10"/></svg>`
}

var testIQKeywords = iqBuiltinSVGQuestions[0].Keywords

func TestExtractIQSVG(t *testing.T) {
	inner := `<svg viewBox="0 0 10 10"><circle r="1"/></svg>`
	cases := []string{
		inner,
		"Here you go:\n```svg\n" + inner + "\n```\nEnjoy",
		"<!DOCTYPE html><html><body>" + inner + "</body></html>",
	}
	for _, c := range cases {
		if got := ExtractIQSVG(c); got != inner {
			t.Fatalf("extract failed: %q -> %q", c, got)
		}
	}
	if ExtractIQSVG("no svg here") != "" {
		t.Fatal("expected empty")
	}
}

func TestAnalyzeIQSVGHardFails(t *testing.T) {
	rng := mathrand.New(mathrand.NewPCG(3, 4))
	if a := AnalyzeIQSVG(richIQSVG(rng), testIQKeywords); len(a.HardFails) != 0 {
		t.Fatalf("rich svg should pass hard checks: %v", a.HardFails)
	}
	cases := map[string]string{
		"no_animation":      `<svg viewBox="0 0 10 10"><circle r="1"/><circle r="2"/><rect width="1" height="1"/><path d="M0 0 L1 1"/><line x2="1"/></svg>`,
		"contains_script":   strings.Replace(simpleIQSVG(), "</svg>", "<script>alert(1)</script></svg>", 1),
		"external_resource": strings.Replace(simpleIQSVG(), "</svg>", `<image href="https://evil.example/a.png"/></svg>`, 1),
		"too_few_shapes":    `<svg viewBox="0 0 10 10"><style>@keyframes a{}</style><circle r="1"/></svg>`,
		"no_svg":            "I cannot draw",
	}
	for want, svg := range cases {
		a := AnalyzeIQSVG(svg, testIQKeywords)
		found := false
		for _, f := range a.HardFails {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected hard fail, got %v", want, a.HardFails)
		}
	}
}

func buildTestIQRefSet(t *testing.T, n int) *IQSVGReferenceSet {
	rng := mathrand.New(mathrand.NewPCG(5, 6))
	var refs []*IQSVGAnalysis
	for i := 0; i < n; i++ {
		refs = append(refs, AnalyzeIQSVG(richIQSVG(rng), testIQKeywords))
	}
	set := BuildIQSVGReferenceSet(refs, "question")
	if set == nil {
		t.Fatal("expected reference set")
	}
	return set
}

func TestIQSVGReferenceScoring(t *testing.T) {
	if BuildIQSVGReferenceSet(nil, "question") != nil {
		t.Fatal("empty set should be nil")
	}
	set := buildTestIQRefSet(t, 20)
	rng := mathrand.New(mathrand.NewPCG(7, 8))
	passed := 0
	for i := 0; i < 20; i++ {
		s := set.Score(AnalyzeIQSVG(richIQSVG(rng), testIQKeywords))
		if s.Verdict() == IQVerdictPassed {
			passed++
		}
	}
	if passed < 17 {
		t.Fatalf("too many rich candidates rejected: %d/20 passed", passed)
	}
	simple := set.Score(AnalyzeIQSVG(simpleIQSVG(), testIQKeywords))
	if simple.Verdict() != IQVerdictFailed {
		t.Fatalf("simple svg should fail, got %s (distance %.2f, thresholds %.2f/%.2f) worst=%v",
			simple.Verdict(), simple.Distance, simple.PassThreshold, simple.FailThreshold, simple.WorstFeatures)
	}
	if simple.Score100() >= 60 {
		t.Fatalf("simple svg display score too high: %v", simple.Score100())
	}
}

func TestIQLogisticModelTrains(t *testing.T) {
	rng := mathrand.New(mathrand.NewPCG(9, 10))
	var good, bad [][]float64
	for i := 0; i < 15; i++ {
		good = append(good, AnalyzeIQSVG(richIQSVG(rng), testIQKeywords).Vector())
		// 负样本：简陋图形的随机变体
		svg := strings.Replace(simpleIQSVG(), `r="50"`, fmt.Sprintf(`r="%d"`, 30+rng.IntN(40)), 1)
		bad = append(bad, AnalyzeIQSVG(svg, testIQKeywords).Vector())
	}
	m, acc := TrainIQLogisticModel(good, bad)
	if m == nil || acc < 0.9 {
		t.Fatalf("expected trained model with high accuracy, got %v", acc)
	}
	raw, _ := json.Marshal(m)
	var back IQLogisticModel
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	p, ok := back.Predict(AnalyzeIQSVG(richIQSVG(rng), testIQKeywords).Features)
	if !ok || p < 0.5 {
		t.Fatalf("expected good prediction, got %v %v", p, ok)
	}
	if m2, _ := TrainIQLogisticModel(good[:3], bad[:3]); m2 != nil {
		t.Fatal("should refuse to train with too few samples")
	}
}

// ---------------- 汇总与处置 ----------------

func TestAggregateIQVerdict(t *testing.T) {
	mk := func(vs ...string) []*IQTestResult {
		out := make([]*IQTestResult, len(vs))
		for i, v := range vs {
			out[i] = &IQTestResult{Verdict: v}
		}
		return out
	}
	cases := []struct {
		in   []*IQTestResult
		want string
	}{
		{mk(IQVerdictPassed, IQVerdictPassed), IQVerdictPassed},
		{mk(IQVerdictPassed, IQVerdictError), IQVerdictPassed},
		{mk(IQVerdictFailed, IQVerdictPassed), IQVerdictFailed},
		{mk(IQVerdictFailed, IQVerdictPassed, IQVerdictPassed), IQVerdictInconclusive},
		{mk(IQVerdictError, IQVerdictError), IQVerdictError},
		{mk(IQVerdictInconclusive), IQVerdictInconclusive},
	}
	for _, c := range cases {
		if got := AggregateIQVerdict(c.in); got != c.want {
			t.Errorf("got %s want %s", got, c.want)
		}
	}
}

func TestIQApplyRoundOutcomeHealthPenalty(t *testing.T) {
	tracker := NewAccountHealthTracker()
	svc := &IQTestService{health: tracker}
	p := &IQTestPlan{ID: 1, AccountID: 42, FailAction: IQActionHealthPenalty, FailThreshold: 2, QuestionSlug: "candy"}

	svc.applyRoundOutcome(nil, p, IQVerdictFailed)
	if p.ActionApplied || tracker.Snapshot(42).PenaltyFactor != 1 {
		t.Fatal("should not apply before threshold")
	}
	svc.applyRoundOutcome(nil, p, IQVerdictInconclusive)
	svc.applyRoundOutcome(nil, p, IQVerdictFailed)
	if !p.ActionApplied || tracker.Snapshot(42).PenaltyFactor != iqHealthPenaltyValue {
		t.Fatalf("penalty should be applied: %+v", tracker.Snapshot(42))
	}
	svc.applyRoundOutcome(nil, p, IQVerdictPassed)
	if p.ActionApplied || p.ConsecutiveFails != 0 || tracker.Snapshot(42).PenaltyFactor != 1 {
		t.Fatal("pass should revert penalty")
	}
}

func TestIQApplyRoundOutcomeObserveOnly(t *testing.T) {
	tracker := NewAccountHealthTracker()
	svc := &IQTestService{health: tracker}
	p := &IQTestPlan{AccountID: 7, FailAction: IQActionObserveOnly, FailThreshold: 1}
	svc.applyRoundOutcome(nil, p, IQVerdictFailed)
	if p.ActionApplied || p.ConsecutiveFails != 1 {
		t.Fatal("observe_only must not apply actions")
	}
}
