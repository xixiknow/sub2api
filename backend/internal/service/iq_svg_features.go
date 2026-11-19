package service

import (
	"encoding/xml"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 画图题本地评估：SVG 解析、硬门槛与特征向量化（纯 Go，无外部依赖）。

// IQSVGFeatureNames 是数值特征的固定顺序；修改顺序会使已训练分类器失效（需重新训练）。
var IQSVGFeatureNames = []string{
	"n_path", "n_circle", "n_ellipse", "n_rect", "n_line", "n_poly", "n_group", "n_shapes",
	"path_cmds", "max_depth", "distinct_colors", "n_gradients", "n_defs_items", "n_anim_elems",
	"n_keyframes", "n_transforms", "code_len", "numeric_tokens", "keyword_hits", "n_comments",
	"n_ids", "bbox_coverage", "offcanvas_ratio", "spatial_spread", "round_shapes", "n_use",
}

// iqSVGComplexityFeatures 中的特征只惩罚「低于参考分布」（降智模型的典型表现是输出更简陋）。
var iqSVGComplexityFeatures = map[string]bool{
	"n_path": true, "n_circle": true, "n_ellipse": true, "n_rect": true, "n_line": true, "n_poly": true,
	"n_group": true, "n_shapes": true, "path_cmds": true, "max_depth": true, "distinct_colors": true,
	"n_gradients": true, "n_defs_items": true, "n_anim_elems": true, "n_keyframes": true,
	"n_transforms": true, "code_len": true, "numeric_tokens": true, "keyword_hits": true,
	"n_comments": true, "n_ids": true, "round_shapes": true, "n_use": true, "spatial_spread": true,
	"bbox_coverage": true,
}

// IQSVGAnalysis 是一次 SVG 分析结果。
type IQSVGAnalysis struct {
	SVG        string             `json:"-"`
	HardFails  []string           `json:"hard_fails,omitempty"`
	Features   map[string]float64 `json:"features"`
	Tokens     map[string]float64 `json:"-"` // 风格 token 词频
	ViewBox    [4]float64         `json:"viewbox"`
	AnimSource string             `json:"anim_source,omitempty"`
}

// Vector 按 IQSVGFeatureNames 顺序返回特征向量。
func (a *IQSVGAnalysis) Vector() []float64 {
	return iqFeatureVector(a.Features)
}

func iqFeatureVector(m map[string]float64) []float64 {
	v := make([]float64, len(IQSVGFeatureNames))
	for i, name := range IQSVGFeatureNames {
		v[i] = m[name]
	}
	return v
}

var (
	iqCodeFencePattern  = regexp.MustCompile("(?s)```[a-zA-Z]*\\s*(.*?)```")
	iqSVGOpenPattern    = regexp.MustCompile(`(?i)<svg[\s>]`)
	iqSVGClosePattern   = regexp.MustCompile(`(?i)</svg\s*>`)
	iqExternalRefRegexp = regexp.MustCompile(`(?i)(?:href\s*=\s*["']\s*(?:https?:)?//|url\(\s*["']?\s*(?:https?:)?//|@import)`)
	iqNumberPattern     = regexp.MustCompile(`[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)
	iqPathCmdPattern    = regexp.MustCompile(`[MmLlHhVvCcSsQqTtAaZz]`)
	iqColorPattern      = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|rgba?\([^)]*\)|hsla?\([^)]*\)`)
	iqKeyframesPattern  = regexp.MustCompile(`(?i)@keyframes\s+[\w-]+`)
	iqCSSPropPattern    = regexp.MustCompile(`(?i)([a-z-]+)\s*:`)
	iqEventAttrPattern  = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
)

var iqNamedColors = map[string]bool{
	"black": true, "white": true, "red": true, "green": true, "blue": true, "yellow": true, "orange": true,
	"gray": true, "grey": true, "pink": true, "purple": true, "brown": true, "gold": true, "silver": true,
	"navy": true, "teal": true, "skyblue": true, "lightblue": true, "darkgray": true, "lightgray": true,
}

// ExtractIQSVG 从模型输出中取出 SVG 源码（支持 Markdown 代码块与 HTML 包裹）。
func ExtractIQSVG(output string) string {
	text := stripIQThinking(output)
	if m := iqCodeFencePattern.FindAllStringSubmatch(text, -1); len(m) > 0 {
		for _, g := range m {
			if iqSVGOpenPattern.MatchString(g[1]) {
				text = g[1]
				break
			}
		}
	}
	start := iqSVGOpenPattern.FindStringIndex(text)
	if start == nil {
		return ""
	}
	closes := iqSVGClosePattern.FindAllStringIndex(text, -1)
	if len(closes) == 0 {
		return strings.TrimSpace(text[start[0]:])
	}
	end := closes[len(closes)-1][1]
	if end <= start[0] {
		return ""
	}
	return strings.TrimSpace(text[start[0]:end])
}

type iqShapeBox struct{ minX, minY, maxX, maxY float64 }

// AnalyzeIQSVG 解析 SVG，执行硬门槛检查并提取特征。keywords 为题目部件词。
func AnalyzeIQSVG(output string, keywords []string) *IQSVGAnalysis {
	a := &IQSVGAnalysis{Features: map[string]float64{}, Tokens: map[string]float64{}}
	svg := ExtractIQSVG(output)
	a.SVG = svg
	if svg == "" {
		a.HardFails = append(a.HardFails, "no_svg")
		return a
	}
	lower := strings.ToLower(svg)
	if strings.Contains(lower, "<script") || iqEventAttrPattern.MatchString(svg) {
		a.HardFails = append(a.HardFails, "contains_script")
	}
	if iqExternalRefRegexp.MatchString(svg) {
		a.HardFails = append(a.HardFails, "external_resource")
	}

	f := a.Features
	colors := map[string]bool{}
	keywordSet := map[string]bool{}
	for _, k := range keywords {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			keywordSet[k] = true
		}
	}
	keywordHits := map[string]bool{}
	matchKeywords := func(text string) {
		low := strings.ToLower(text)
		for k := range keywordSet {
			if strings.Contains(low, k) {
				keywordHits[k] = true
			}
		}
	}
	var styleText strings.Builder
	var boxes []iqShapeBox
	viewBoxSet := false
	depth, maxDepth := 0, 0
	var stack []string

	dec := xml.NewDecoder(strings.NewReader(svg))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	parseErr := false
	inStyle := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErr = true
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
			parent := "root"
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, name)
			a.Tokens["el:"+name]++
			a.Tokens["pc:"+parent+">"+name]++
			attrs := map[string]string{}
			for _, at := range t.Attr {
				an := strings.ToLower(at.Name.Local)
				attrs[an] = at.Value
				a.Tokens["at:"+an]++
				switch an {
				case "fill", "stroke", "stop-color", "color":
					addIQColor(colors, at.Value)
				case "style":
					styleText.WriteString(at.Value)
					styleText.WriteString(";")
				case "id", "class":
					matchKeywords(at.Value)
					if an == "id" {
						f["n_ids"]++
					}
				case "transform":
					f["n_transforms"]++
				}
			}
			switch name {
			case "svg":
				if !viewBoxSet {
					if vb, ok := parseIQViewBox(attrs); ok {
						a.ViewBox = vb
						viewBoxSet = true
					}
				}
			case "path":
				f["n_path"]++
				d := attrs["d"]
				f["path_cmds"] += float64(len(iqPathCmdPattern.FindAllString(d, -1)))
				if b, ok := iqPathBox(d); ok {
					boxes = append(boxes, b)
				}
			case "circle":
				f["n_circle"]++
				cx, cy, r := iqAttrF(attrs, "cx"), iqAttrF(attrs, "cy"), iqAttrF(attrs, "r")
				if r > 0 {
					boxes = append(boxes, iqShapeBox{cx - r, cy - r, cx + r, cy + r})
					f["round_shapes"]++
				}
			case "ellipse":
				f["n_ellipse"]++
				cx, cy, rx, ry := iqAttrF(attrs, "cx"), iqAttrF(attrs, "cy"), iqAttrF(attrs, "rx"), iqAttrF(attrs, "ry")
				if rx > 0 && ry > 0 {
					boxes = append(boxes, iqShapeBox{cx - rx, cy - ry, cx + rx, cy + ry})
					if math.Max(rx, ry)/math.Min(rx, ry) < 1.5 {
						f["round_shapes"]++
					}
				}
			case "rect":
				f["n_rect"]++
				x, y, w, h := iqAttrF(attrs, "x"), iqAttrF(attrs, "y"), iqAttrF(attrs, "width"), iqAttrF(attrs, "height")
				if w > 0 && h > 0 {
					boxes = append(boxes, iqShapeBox{x, y, x + w, y + h})
				}
			case "line":
				f["n_line"]++
				x1, y1, x2, y2 := iqAttrF(attrs, "x1"), iqAttrF(attrs, "y1"), iqAttrF(attrs, "x2"), iqAttrF(attrs, "y2")
				boxes = append(boxes, iqShapeBox{math.Min(x1, x2), math.Min(y1, y2), math.Max(x1, x2), math.Max(y1, y2)})
			case "polygon", "polyline":
				f["n_poly"]++
				if b, ok := iqPointsBox(attrs["points"]); ok {
					boxes = append(boxes, b)
				}
			case "g":
				f["n_group"]++
			case "lineargradient", "radialgradient":
				f["n_gradients"]++
			case "filter", "clippath", "mask", "pattern", "symbol", "marker":
				f["n_defs_items"]++
			case "animate", "animatetransform", "animatemotion", "set":
				f["n_anim_elems"]++
			case "use":
				f["n_use"]++
			case "style":
				inStyle = true
			}
		case xml.EndElement:
			depth--
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if strings.EqualFold(t.Name.Local, "style") {
				inStyle = false
			}
		case xml.CharData:
			if inStyle {
				styleText.Write(t)
			}
		case xml.Comment:
			f["n_comments"]++
			matchKeywords(string(t))
		}
	}
	if parseErr {
		a.HardFails = append(a.HardFails, "invalid_xml")
	}

	css := styleText.String()
	f["n_keyframes"] = float64(len(iqKeyframesPattern.FindAllString(css, -1)))
	for _, m := range iqCSSPropPattern.FindAllStringSubmatch(css, -1) {
		a.Tokens["css:"+strings.ToLower(m[1])]++
	}
	for _, c := range iqColorPattern.FindAllString(css, -1) {
		addIQColor(colors, c)
	}
	matchKeywords(css)
	f["distinct_colors"] = float64(len(colors))
	f["keyword_hits"] = float64(len(keywordHits))
	f["n_shapes"] = f["n_path"] + f["n_circle"] + f["n_ellipse"] + f["n_rect"] + f["n_line"] + f["n_poly"]
	f["max_depth"] = float64(maxDepth)
	f["code_len"] = float64(len(svg))
	f["numeric_tokens"] = float64(len(iqNumberPattern.FindAllStringIndex(svg, -1)))

	switch {
	case f["n_anim_elems"] > 0:
		a.AnimSource = "smil"
	case f["n_keyframes"] > 0:
		a.AnimSource = "css"
	}
	if a.AnimSource == "" {
		a.HardFails = append(a.HardFails, "no_animation")
	}
	if f["n_shapes"] < 5 {
		a.HardFails = append(a.HardFails, "too_few_shapes")
	}

	// 几何特征（近似：忽略 transform）
	vb := a.ViewBox
	if !viewBoxSet || vb[2] <= 0 || vb[3] <= 0 {
		vb = iqUnionBox(boxes, [4]float64{0, 0, 800, 600})
		a.ViewBox = vb
	}
	if len(boxes) > 0 {
		cov, off, spread := iqGeometry(boxes, vb)
		f["bbox_coverage"] = cov
		f["offcanvas_ratio"] = off
		f["spatial_spread"] = spread
	}

	// 计数类特征取 log1p，压缩量级差异
	for _, name := range IQSVGFeatureNames {
		switch name {
		case "bbox_coverage", "offcanvas_ratio", "spatial_spread":
			continue
		}
		f[name] = math.Log1p(f[name])
	}
	return a
}

func addIQColor(colors map[string]bool, v string) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" || v == "none" || v == "transparent" || strings.HasPrefix(v, "url(") || v == "currentcolor" || v == "inherit" {
		return
	}
	if strings.HasPrefix(v, "#") || strings.HasPrefix(v, "rgb") || strings.HasPrefix(v, "hsl") || iqNamedColors[v] {
		colors[v] = true
	}
}

func iqAttrF(attrs map[string]string, key string) float64 {
	s := strings.TrimSpace(attrs[key])
	s = strings.TrimSuffix(s, "px")
	if s == "" || strings.HasSuffix(s, "%") {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func parseIQViewBox(attrs map[string]string) ([4]float64, bool) {
	var vb [4]float64
	if raw := strings.TrimSpace(attrs["viewbox"]); raw != "" {
		nums := iqNumberPattern.FindAllString(raw, -1)
		if len(nums) == 4 {
			for i, n := range nums {
				vb[i], _ = strconv.ParseFloat(n, 64)
			}
			if vb[2] > 0 && vb[3] > 0 {
				return vb, true
			}
		}
	}
	w, h := iqAttrF(attrs, "width"), iqAttrF(attrs, "height")
	if w > 0 && h > 0 {
		return [4]float64{0, 0, w, h}, true
	}
	return vb, false
}

func iqNumbers(s string) []float64 {
	raw := iqNumberPattern.FindAllString(s, -1)
	out := make([]float64, 0, len(raw))
	for _, r := range raw {
		if v, err := strconv.ParseFloat(r, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func iqPointsBox(points string) (iqShapeBox, bool) {
	nums := iqNumbers(points)
	if len(nums) < 4 {
		return iqShapeBox{}, false
	}
	b := iqShapeBox{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for i := 0; i+1 < len(nums); i += 2 {
		b.minX, b.maxX = math.Min(b.minX, nums[i]), math.Max(b.maxX, nums[i])
		b.minY, b.maxY = math.Min(b.minY, nums[i+1]), math.Max(b.maxY, nums[i+1])
	}
	return b, true
}

// iqPathBox 只对纯绝对坐标的 M/L/C/Q/S/T 路径估算包围盒（含相对命令/弧线时跳过）。
func iqPathBox(d string) (iqShapeBox, bool) {
	if strings.ContainsAny(d, "mlhvcsqtaHVA") {
		return iqShapeBox{}, false
	}
	return iqPointsBox(d)
}

func iqUnionBox(boxes []iqShapeBox, fallback [4]float64) [4]float64 {
	if len(boxes) == 0 {
		return fallback
	}
	u := boxes[0]
	for _, b := range boxes[1:] {
		u.minX, u.minY = math.Min(u.minX, b.minX), math.Min(u.minY, b.minY)
		u.maxX, u.maxY = math.Max(u.maxX, b.maxX), math.Max(u.maxY, b.maxY)
	}
	if u.maxX-u.minX <= 0 || u.maxY-u.minY <= 0 {
		return fallback
	}
	return [4]float64{u.minX, u.minY, u.maxX - u.minX, u.maxY - u.minY}
}

// iqGeometry 返回 画布覆盖率、画布外元素比例、元素中心离散度（相对画布对角线）。
func iqGeometry(boxes []iqShapeBox, vb [4]float64) (coverage, offcanvas, spread float64) {
	x0, y0, x1, y1 := vb[0], vb[1], vb[0]+vb[2], vb[1]+vb[3]
	area := vb[2] * vb[3]
	if area <= 0 {
		return 0, 0, 0
	}
	u := iqUnionBox(boxes, vb)
	ix0, iy0 := math.Max(u[0], x0), math.Max(u[1], y0)
	ix1, iy1 := math.Min(u[0]+u[2], x1), math.Min(u[1]+u[3], y1)
	if ix1 > ix0 && iy1 > iy0 {
		coverage = (ix1 - ix0) * (iy1 - iy0) / area
	}
	off := 0
	var sx, sy, sxx, syy float64
	for _, b := range boxes {
		cx, cy := (b.minX+b.maxX)/2, (b.minY+b.maxY)/2
		if cx < x0 || cx > x1 || cy < y0 || cy > y1 {
			off++
		}
		sx += cx
		sy += cy
		sxx += cx * cx
		syy += cy * cy
	}
	n := float64(len(boxes))
	offcanvas = float64(off) / n
	varX := sxx/n - (sx/n)*(sx/n)
	varY := syy/n - (sy/n)*(sy/n)
	diag := math.Hypot(vb[2], vb[3])
	if diag > 0 {
		spread = math.Sqrt(math.Max(varX, 0)+math.Max(varY, 0)) / diag
	}
	return clamp01(coverage), offcanvas, spread
}

// iqTopTokens 返回词频最高的 token（用于调试展示）。
func iqTopTokens(tokens map[string]float64, n int) []string {
	keys := make([]string, 0, len(tokens))
	for k := range tokens {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if tokens[keys[i]] != tokens[keys[j]] {
			return tokens[keys[i]] > tokens[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	return keys
}
