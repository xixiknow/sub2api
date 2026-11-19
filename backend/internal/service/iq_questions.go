package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// 模型智力测试（fork 扩展）：题型与判分器。
//
// 题型：
//   - candy：固定逻辑题，答案 21，正则判分（移植自 ranxi 版 candy_answer.go）
//   - random_reasoning：程序生成 + 程序求解的推理题（格点路径 / 加权 XOR），每次不同，防缓存作弊
//   - svg_draw：画图题（鹈鹕骑车等），基于参考样本的本地特征评估（见 iq_svg_*.go）

const (
	IQKindCandy           = "candy"
	IQKindRandomReasoning = "random_reasoning"
	IQKindSVGDraw         = "svg_draw"
)

const iqCandyPrompt = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）
苹果味 桃子味 西瓜味
圆形 7 9 8
五角星形 7 6 4`

const iqAnswerSuffix = "\n只输出最终答案，不要解释。"

// iqSVGDeliveryContract 要求输出可被服务端解析的独立 SVG。
const iqSVGDeliveryContract = "\n交付要求：只输出一个完整、独立的 <svg> 元素（必须带 viewBox），动画用 SVG 内的 <style> CSS @keyframes 或 SMIL <animate>/<animateTransform> 实现；不要 JavaScript、不要外部资源、不要 Markdown 代码块，不要任何解释。"

// IQSVGQuestionPreset 是内置画图题。
type IQSVGQuestionPreset struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Prompt   string   `json:"prompt"`
	Keywords []string `json:"keywords"` // 部件词，用于语义特征（id/class/注释命中）
}

var iqBuiltinSVGQuestions = []IQSVGQuestionPreset{
	{"pelican-bicycle", "鹈鹕骑自行车", "创建一个 SVG 动画：一只鹈鹕骑自行车。画出长喙和喉囊、双轮车架、脚踏与腿部联动，车轮转动，背景路标向后移动。",
		[]string{"pelican", "beak", "pouch", "bill", "wing", "eye", "leg", "wheel", "spoke", "pedal", "frame", "handle", "seat", "chain", "road", "sign", "鹈鹕", "喙", "车轮", "踏板"}},
	{"penguin-bicycle", "企鹅骑自行车", "创建一个 SVG 动画：一只企鹅骑自行车。画出黑白身体和短喙、双轮车架、脚踏与双脚联动，车轮转动，背景冰块向后移动。",
		[]string{"penguin", "beak", "belly", "flipper", "wing", "eye", "foot", "feet", "wheel", "spoke", "pedal", "frame", "handle", "seat", "ice", "企鹅", "车轮", "踏板"}},
	{"rabbit-scooter", "兔子骑滑板车", "创建一个 SVG 动画：一只兔子骑双轮滑板车。画出长耳、踏板和车把，一脚站立、另一脚蹬地后收回，车轮转动，背景草丛向后移动。",
		[]string{"rabbit", "bunny", "ear", "tail", "eye", "leg", "foot", "wheel", "deck", "board", "handle", "bar", "scooter", "grass", "兔", "车轮", "滑板"}},
	{"otter-scooter", "水獭骑滑板车", "创建一个 SVG 动画：一只水獭骑双轮滑板车。画出圆耳和长尾、踏板和车把，一脚站立、另一脚蹬地后收回，车轮转动，背景石块向后移动。",
		[]string{"otter", "ear", "tail", "whisker", "eye", "leg", "foot", "wheel", "deck", "board", "handle", "bar", "scooter", "rock", "stone", "水獭", "车轮", "滑板"}},
}

// IQBuiltinSVGQuestions 返回内置画图题（只读副本）。
func IQBuiltinSVGQuestions() []IQSVGQuestionPreset {
	out := make([]IQSVGQuestionPreset, len(iqBuiltinSVGQuestions))
	copy(out, iqBuiltinSVGQuestions)
	return out
}

// ---------------- candy ----------------

var iqCandyAnswerPattern = regexp.MustCompile(`^(?:(?:最终)?(?:答案|结果)(?:为|是)?[:：]?|(?:最少|至少)(?:需要)?(?:取出|抽取|摸出)?|(?:需要|取出|抽取|摸出))?(?:21|二十一)(?:个|颗|粒)?(?:糖果|糖)?(?:即可)?[。.!！]?$|^(?:(?:the)?answer(?:is|:)?|atleast)?21(?:candies)?[.!]?$`)
var iqCandySplitNumberPattern = regexp.MustCompile(`[0-9０-９][\s\p{Zs}*_]+[0-9０-９]`)

// IQCandyAnswerCorrect 判断糖果题答案是否为 21（允许常见前后缀、全角数字、中文数字）。
func IQCandyAnswerCorrect(output string) bool {
	output = stripIQThinking(output)
	if iqCandySplitNumberPattern.MatchString(output) {
		return false
	}
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("*_`\"'“”", r) {
			return -1
		}
		if r >= '０' && r <= '９' {
			return '0' + r - '０'
		}
		return unicode.ToLower(r)
	}, output)
	return iqCandyAnswerPattern.MatchString(normalized)
}

var iqThinkBlockPattern = regexp.MustCompile(`(?s)<think>.*?</think>`)

// stripIQThinking 去掉部分中转站把推理过程内联输出的 <think> 块。
func stripIQThinking(s string) string {
	return strings.TrimSpace(iqThinkBlockPattern.ReplaceAllString(s, ""))
}

// ---------------- random_reasoning ----------------

// IQRandomQuestion 是一道随机生成的推理题，Reference 为程序求解出的标准答案（JSON）。
type IQRandomQuestion struct {
	ID        string `json:"id"`
	Family    string `json:"family"` // paths | weighted_xor
	Prompt    string `json:"prompt"`
	Reference string `json:"reference"`
}

// GenerateIQRandomQuestion 用 crypto/rand 种子生成一道新题。
func GenerateIQRandomQuestion() (*IQRandomQuestion, error) {
	return generateIQRandomQuestion(rand.Reader, -1)
}

func generateIQRandomQuestion(source io.Reader, family int) (*IQRandomQuestion, error) {
	var seed [16]byte
	if _, err := io.ReadFull(source, seed[:]); err != nil {
		return nil, fmt.Errorf("random question entropy unavailable")
	}
	rng := mathrand.New(mathrand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:]))) //nolint:gosec
	if family < 0 {
		family = rng.IntN(2)
	}
	var (
		prompt, name string
		answer       any
		err          error
	)
	if family == 0 {
		name = "paths"
		prompt, answer, err = generateIQPathQuestion(rng)
	} else {
		name = "weighted_xor"
		prompt, answer, err = generateIQXORQuestion(rng)
	}
	if err != nil {
		return nil, err
	}
	ref, err := json.Marshal(answer)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(prompt))
	return &IQRandomQuestion{ID: fmt.Sprintf("%x", digest[:12]), Family: name, Prompt: prompt, Reference: string(ref)}, nil
}

type iqGridPoint struct{ X, Y int }

type iqPathProblem struct {
	Width, Height, Turns int
	Blocked              [3]iqGridPoint
	Checkpoints          [2]iqGridPoint
}

type iqPathAnswer struct {
	Count int    `json:"count"`
	First string `json:"first"`
}

type iqPathState struct{ X, Y, Last, Turns, Hits, Run int }

func solveIQPath(p iqPathProblem) iqPathAnswer {
	memo := make(map[iqPathState]int)
	blocked := func(x, y int) bool {
		for _, point := range p.Blocked {
			if point.X == x && point.Y == y {
				return true
			}
		}
		return false
	}
	next := func(state iqPathState, direction int) iqPathState {
		if direction == 1 {
			state.X++
			state.Run++
		} else {
			state.Y++
			state.Run = 0
		}
		if state.Last != 0 && state.Last != direction {
			state.Turns++
		}
		state.Last = direction
		for _, point := range p.Checkpoints {
			if point.X == state.X && point.Y == state.Y {
				state.Hits++
			}
		}
		return state
	}
	var count func(iqPathState) int
	count = func(state iqPathState) int {
		if state.X > p.Width || state.Y > p.Height || state.Turns > p.Turns || state.Hits > 1 || state.Run >= 3 || blocked(state.X, state.Y) {
			return 0
		}
		if state.X == p.Width && state.Y == p.Height {
			if state.Turns == p.Turns && state.Hits == 1 {
				return 1
			}
			return 0
		}
		if value, ok := memo[state]; ok {
			return value
		}
		value := count(next(state, 1)) + count(next(state, 2))
		memo[state] = value
		return value
	}
	state := iqPathState{}
	answer := iqPathAnswer{Count: count(state)}
	if answer.Count == 0 {
		return answer
	}
	var first strings.Builder
	for state.X != p.Width || state.Y != p.Height {
		if right := next(state, 1); count(right) > 0 {
			_ = first.WriteByte('R')
			state = right
		} else {
			_ = first.WriteByte('U')
			state = next(state, 2)
		}
	}
	answer.First = first.String()
	return answer
}

func generateIQPathQuestion(rng *mathrand.Rand) (string, any, error) {
	for attempt := 0; attempt < 128; attempt++ {
		p := iqPathProblem{Width: 5 + rng.IntN(3), Height: 5 + rng.IntN(3), Turns: 4 + rng.IntN(4)}
		var points []iqGridPoint
		for x := 0; x <= p.Width; x++ {
			for y := 0; y <= p.Height; y++ {
				if (x != 0 || y != 0) && (x != p.Width || y != p.Height) {
					points = append(points, iqGridPoint{x, y})
				}
			}
		}
		order := rng.Perm(len(points))
		for i := range p.Blocked {
			p.Blocked[i] = points[order[i]]
		}
		for i := range p.Checkpoints {
			p.Checkpoints[i] = points[order[i+3]]
		}
		answer := solveIQPath(p)
		if answer.Count < 3 {
			continue
		}
		pt := func(point iqGridPoint) string { return fmt.Sprintf("(%d,%d)", point.X, point.Y) }
		prompt := fmt.Sprintf("从格点(0,0)走到(%d,%d)，每步只能R=(1,0)或U=(0,1)。不能经过禁点%s、%s、%s；检查点%s、%s必须恰好经过一个。相邻动作字母不同时算一次转向，第一步不算，必须恰好转向%d次。动作串不得包含连续三个R。求合法动作串总数及字典序最小的动作串，规定R<U。只输出JSON对象，字段count为整数，first为动作串，不加解释或代码块。",
			p.Width, p.Height, pt(p.Blocked[0]), pt(p.Blocked[1]), pt(p.Blocked[2]), pt(p.Checkpoints[0]), pt(p.Checkpoints[1]), p.Turns)
		return prompt, answer, nil
	}
	return "", nil, fmt.Errorf("random path question unavailable")
}

type iqXORObservation struct {
	Indices []int
	Value   int
	Weight  int
}

type iqXORAnswer struct {
	Penalty     int      `json:"penalty"`
	Assignments []string `json:"assignments"`
}

func solveIQXOR(rows []iqXORObservation) iqXORAnswer {
	answer := iqXORAnswer{Penalty: int(^uint(0) >> 1)}
	for assignment := 0; assignment < 64; assignment++ {
		bits := make([]byte, 6)
		for i := range bits {
			bits[i] = '0' + byte(assignment>>(5-i)&1)
		}
		penalty := 0
		for _, row := range rows {
			value := 0
			for _, index := range row.Indices {
				value ^= int(bits[index] - '0')
			}
			if value != row.Value {
				penalty += row.Weight
			}
		}
		if penalty < answer.Penalty {
			answer.Penalty = penalty
			answer.Assignments = nil
		}
		if penalty == answer.Penalty {
			answer.Assignments = append(answer.Assignments, string(bits))
		}
	}
	return answer
}

func generateIQXORQuestion(rng *mathrand.Rand) (string, any, error) {
	indices := [][]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 5}, {0, 5}, {0, 2, 4}, {1, 3, 5}, {0, 3, 4}, {1, 2, 5}, {0, 1, 5}}
	for attempt := 0; attempt < 128; attempt++ {
		truth := rng.IntN(64)
		order := rng.Perm(len(indices))
		corrupt := map[int]bool{order[0]: true, order[1]: true, order[2]: true}
		rows := make([]iqXORObservation, len(indices))
		for i, members := range indices {
			value := 0
			for _, index := range members {
				// bits 字符串从左到右对应 A..F，即 bit(5-index)
				value ^= truth >> (5 - index) & 1
			}
			if corrupt[i] {
				value ^= 1
			}
			rows[i] = iqXORObservation{Indices: members, Value: value, Weight: 1 + rng.IntN(6)}
		}
		answer := solveIQXOR(rows)
		if answer.Penalty == 0 || len(answer.Assignments) > 4 {
			continue
		}
		var prompt strings.Builder
		_, _ = prompt.WriteString("寄存器A,B,C,D,E,F各取0或1。以下观测可能有误，XOR为异或，即各位之和模2。总代价为所有不成立观测的违背代价之和；不限制错误条数，不是让错误条数最少。\n")
		for i, row := range rows {
			members := make([]string, len(row.Indices))
			for j, index := range row.Indices {
				members[j] = string(rune('A' + index))
			}
			fmt.Fprintf(&prompt, "%d. %s = %d；违背代价%d\n", i+1, strings.Join(members, " XOR "), row.Value, row.Weight)
		}
		_, _ = prompt.WriteString("求最小总代价及所有最优赋值。只输出JSON对象，字段penalty为整数，assignments为六位01字符串数组，从左至右对应ABCDEF，按字典序升序。不加解释或代码块。")
		return prompt.String(), answer, nil
	}
	return "", nil, fmt.Errorf("random XOR question unavailable")
}

var iqJSONObjectPattern = regexp.MustCompile(`(?s)\{.*\}`)

// IQRandomAnswerCorrect 比对模型输出与标准答案（只比较关键字段，assignments 按集合比较）。
func IQRandomAnswerCorrect(family, reference, output string) (bool, string) {
	output = stripIQThinking(output)
	raw := iqJSONObjectPattern.FindString(output)
	if raw == "" {
		return false, "no_json_object"
	}
	switch family {
	case "paths":
		var want, got iqPathAnswer
		if err := json.Unmarshal([]byte(reference), &want); err != nil {
			return false, "bad_reference"
		}
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			return false, "invalid_json"
		}
		if got.Count != want.Count {
			return false, fmt.Sprintf("count_mismatch: want %d got %d", want.Count, got.Count)
		}
		if !strings.EqualFold(strings.TrimSpace(got.First), want.First) {
			return false, fmt.Sprintf("first_mismatch: want %s got %s", want.First, got.First)
		}
		return true, ""
	case "weighted_xor":
		var want, got iqXORAnswer
		if err := json.Unmarshal([]byte(reference), &want); err != nil {
			return false, "bad_reference"
		}
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			return false, "invalid_json"
		}
		if got.Penalty != want.Penalty {
			return false, fmt.Sprintf("penalty_mismatch: want %d got %d", want.Penalty, got.Penalty)
		}
		a := append([]string(nil), want.Assignments...)
		b := make([]string, 0, len(got.Assignments))
		for _, s := range got.Assignments {
			b = append(b, strings.TrimSpace(s))
		}
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, ",") != strings.Join(b, ",") {
			return false, fmt.Sprintf("assignments_mismatch: want %v got %v", a, b)
		}
		return true, ""
	}
	return false, "unknown_family"
}
