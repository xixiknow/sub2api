package render

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"time"
)

// Status levels used by the renderer (service maps monitor statuses onto these).
const (
	LevelOK      = "ok"
	LevelWarn    = "degraded"
	LevelDown    = "down"
	LevelUnknown = "unknown"
)

// StatusChannel is one channel card.
type StatusChannel struct {
	Platform       string // section title (display name); empty → "其他"
	Name           string
	GroupName      string
	Model          string
	Level          string
	LatencyMs      *int
	Availability7d float64  // 0-100; negative = no data
	Timeline       []string // levels, oldest first
}

type StatusInput struct {
	Title     string // default "渠道状态"
	UpdatedAt time.Time
	Channels  []StatusChannel
	MaxCards  int // default 18
}

const (
	statusW       = 900.0
	statusPad     = 24.0
	statusCols    = 3
	statusGap     = 12.0
	statusCardH   = 112.0
	timelineSlots = 20
	statusLabelH  = 28.0 // platform tag lane above each card row
)

var levelRank = map[string]int{LevelDown: 0, LevelWarn: 1, LevelUnknown: 2, LevelOK: 3}

func levelColors(level string) (dot, fg, bg color.RGBA, label string) {
	switch level {
	case LevelOK:
		return colOK, colOKText, colOKBg, "正常"
	case LevelWarn:
		return colWarn, colWarnText, colWarnBg, "降级"
	case LevelDown:
		return colDown, colDownText, colDownBg, "故障"
	default:
		return colUnknown, colMuted, colUnkBg, "无数据"
	}
}

type statusSection struct {
	platform string
	channels []StatusChannel
	counts   map[string]int
	total    int // before the card budget is applied
}

// groupStatusSections keeps platforms in first-appearance order and sorts cards
// inside each section by severity (down → degraded → unknown → ok).
func groupStatusSections(chs []StatusChannel) []*statusSection {
	var out []*statusSection
	idx := map[string]*statusSection{}
	for _, ch := range chs {
		p := ch.Platform
		if p == "" {
			p = "其他"
		}
		sec, ok := idx[p]
		if !ok {
			sec = &statusSection{platform: p, counts: map[string]int{}}
			idx[p] = sec
			out = append(out, sec)
		}
		sec.channels = append(sec.channels, ch)
		sec.counts[ch.Level]++
		sec.total++
	}
	for _, sec := range out {
		sort.SliceStable(sec.channels, func(i, j int) bool {
			return levelRank[sec.channels[i].Level] < levelRank[sec.channels[j].Level]
		})
	}
	return out
}

// RenderStatus draws the channel status cards, sectioned by platform, as PNG.
func RenderStatus(in StatusInput) ([]byte, error) {
	if err := loadFonts(); err != nil {
		return nil, err
	}
	if in.Title == "" {
		in.Title = "渠道状态"
	}
	if in.MaxCards <= 0 {
		in.MaxCards = 18
	}
	counts := map[string]int{}
	for _, ch := range in.Channels {
		counts[ch.Level]++
	}
	sections := groupStatusSections(in.Channels)
	// Apply the card budget across sections in order; remember what was cut.
	hidden, budget := 0, in.MaxCards
	for _, sec := range sections {
		if len(sec.channels) > budget {
			hidden += len(sec.channels) - budget
			sec.channels = sec.channels[:budget]
		}
		budget -= len(sec.channels)
	}

	// Flow every card into one grid (platform order kept); each row reserves a
	// label lane where a small platform tag sits above that platform's cards.
	var slots []statusSlot
	for _, sec := range sections {
		for _, ch := range sec.channels {
			slots = append(slots, statusSlot{sec: sec, ch: ch})
		}
	}
	rows := int(math.Ceil(float64(len(slots)) / statusCols))
	rowH := statusLabelH + statusCardH
	headerH, footerH := 64.0, 40.0
	bodyH := float64(rows)*rowH + float64(max(rows-1, 0))*statusGap
	if rows == 0 {
		bodyH = 80
	}
	h := statusPad + headerH + bodyH + footerH + statusPad/2
	c := newCanvas(statusW, h, colPage)

	// Header: title + time on the left, counters on the right.
	x, y := statusPad, statusPad
	c.text(x, y+24, in.Title, true, 24, colText)
	c.text(x, y+48, "更新于 "+in.UpdatedAt.Format("01-02 15:04"), false, 13, colMuted)
	rx := statusW - statusPad
	for _, lv := range []string{LevelDown, LevelWarn, LevelOK} {
		dot, fg, bg, label := levelColors(lv)
		s := fmt.Sprintf("%s %d", label, counts[lv])
		w := c.measure(s, true, 13) + 30
		rx -= w
		c.roundRect(rx, y+8, w, 28, 14, bg)
		c.circle(rx+13, y+22, 4.5, dot)
		c.text(rx+22, c.baseline(y+8, 28, true, 13), s, true, 13, fg)
		rx -= 8
	}

	top := statusPad + headerH
	if len(in.Channels) == 0 {
		c.borderRect(statusPad, top, statusW-2*statusPad, bodyH, 12, colCard, colBorder)
		msg := "暂无监控渠道"
		c.text((statusW-c.measure(msg, false, 15))/2, c.baseline(top, bodyH, false, 15), msg, false, 15, colMuted)
	}
	cardW := (statusW - 2*statusPad - (statusCols-1)*statusGap) / statusCols
	for r := 0; r < rows; r++ {
		ry := top + float64(r)*(rowH+statusGap)
		row := slots[r*statusCols : min((r+1)*statusCols, len(slots))]
		// Label each run of same-platform cards in this row.
		for i := 0; i < len(row); {
			j := i
			for j < len(row) && row[j].sec == row[i].sec {
				j++
			}
			lx := statusPad + float64(i)*(cardW+statusGap)
			lw := float64(j-i)*cardW + float64(j-i-1)*statusGap
			drawStatusLabel(c, lx, ry, lw, row[i].sec)
			i = j
		}
		for i, sl := range row {
			drawStatusCard(c, statusPad+float64(i)*(cardW+statusGap), ry+statusLabelH, cardW, sl.ch)
		}
	}

	// Footer: legend + overflow note.
	fy := top + bodyH + 26
	lx := statusPad
	for _, lv := range []string{LevelOK, LevelWarn, LevelDown, LevelUnknown} {
		dot, _, _, label := levelColors(lv)
		c.roundRect(lx, fy-9, 10, 10, 2, dot)
		lx += 14 + c.text(lx+14, fy, label, false, 12, colMuted) + 14
	}
	note := "色条为最近 20 次探测，右侧最新"
	if hidden > 0 {
		note = fmt.Sprintf("另有 %d 个渠道，详见网站", hidden)
	}
	c.textRight(statusW-statusPad, fy, note, false, 12, colFaint)
	return c.png()
}

type statusSlot struct {
	sec *statusSection
	ch  StatusChannel
}

// drawStatusLabel draws a compact platform tag (accent bar, name, count, issues)
// over a run of cards spanning width w, with an accent underline.
func drawStatusLabel(c *canvas, x, y, w float64, sec *statusSection) {
	acc := accentFor(sec.platform, "")
	bl := y + 15
	c.roundRect(x+2, bl-11, 3, 12, 1.5, acc)
	tx := x + 10
	tx += c.text(tx, bl, c.truncate(sec.platform, true, 13, w*0.5), true, 13, acc) + 6
	tx += c.text(tx, bl, fmt.Sprintf("%d", sec.total), false, 11, colFaint) + 10
	for _, lv := range []string{LevelDown, LevelWarn} {
		if n := sec.counts[lv]; n > 0 && tx < x+w-40 {
			_, fg, _, label := levelColors(lv)
			tx += c.text(tx, bl, fmt.Sprintf("%s %d", label, n), true, 11, fg) + 8
		}
	}
	c.roundRect(x, y+statusLabelH-7, w, 2, 1, mix(acc, 0.35))
}

func drawStatusCard(c *canvas, x, y, w float64, ch StatusChannel) {
	const p = 14.0
	c.borderRect(x, y, w, statusCardH, 10, colCard, colBorder)
	dot, fg, bg, label := levelColors(ch.Level)

	// Row 1: dot + name, status pill on the right.
	pillW := c.measure(label, true, 11) + 2*11*0.6
	c.pill(x+w-p-pillW, y+p-1, label, 11, fg, bg)
	c.circle(x+p+5, y+p+9, 5, dot)
	name := c.truncate(ch.Name, true, 15, w-2*p-14-pillW-8)
	c.text(x+p+16, y+p+14.5, name, true, 15, colText)

	// Row 2: group · model.
	sub := ch.GroupName
	if ch.Model != "" {
		if sub != "" {
			sub += " · "
		}
		sub += ch.Model
	}
	if sub == "" {
		sub = "—"
	}
	c.text(x+p, y+p+36, c.truncate(sub, false, 12, w-2*p), false, 12, colMuted)

	// Row 3: latency | 7d availability.
	ry := y + p + 61
	lat := "--"
	if ch.LatencyMs != nil {
		if *ch.LatencyMs >= 1000 {
			lat = fmt.Sprintf("%.1fs", float64(*ch.LatencyMs)/1000)
		} else {
			lat = fmt.Sprintf("%dms", *ch.LatencyMs)
		}
	}
	lx := x + p
	lx += c.text(lx, ry, "延迟 ", false, 12, colFaint)
	lx += c.text(lx, ry, lat, true, 14, colText)
	lx += 14
	c.roundRect(lx-7, ry-12, 1, 14, 0, colBorder)
	lx += c.text(lx, ry, "7日 ", false, 12, colFaint)
	av, avCol := "--", colMuted
	if ch.Availability7d >= 0 {
		av = fmt.Sprintf("%.1f%%", ch.Availability7d)
		switch {
		case ch.Availability7d >= 99:
			avCol = colOKText
		case ch.Availability7d >= 95:
			avCol = colWarnText
		default:
			avCol = colDownText
		}
	}
	c.text(lx, ry, av, true, 14, avCol)

	// Bottom: timeline bars, newest on the right; missing slots are faint.
	tl := ch.Timeline
	if len(tl) > timelineSlots {
		tl = tl[len(tl)-timelineSlots:]
	}
	gap := 2.5
	bw := (w - 2*p - gap*(timelineSlots-1)) / timelineSlots
	by := y + statusCardH - p - 12
	offset := timelineSlots - len(tl)
	for i := 0; i < timelineSlots; i++ {
		col := colUnkBg
		if i >= offset {
			col, _, _, _ = levelColors(tl[i-offset])
		}
		c.roundRect(x+p+float64(i)*(bw+gap), by, bw, 12, 2, col)
	}
}
