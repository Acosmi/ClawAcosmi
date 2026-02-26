// render.go — Markdown IR 渲染器。
//
// TS 对照: markdown/render.ts (196L)
//
// 将 MarkdownIR 按照指定的样式标记和链接构建器渲染为格式化文本。
// 处理样式和链接边界的交叉和嵌套，使用栈式 LIFO 关闭策略。
package markdown

import "sort"

// RenderStyleMarker 样式标记对。
// TS 对照: render.ts RenderStyleMarker
type RenderStyleMarker struct {
	Open  string
	Close string
}

// RenderStyleMap 样式名到标记的映射。
// TS 对照: render.ts RenderStyleMap
type RenderStyleMap map[MarkdownStyle]RenderStyleMarker

// RenderLink 已解析的链接渲染信息。
// TS 对照: render.ts RenderLink
type RenderLink struct {
	Start int
	End   int
	Open  string
	Close string
}

// BuildLinkFn 链接构建函数类型。
type BuildLinkFn func(link MarkdownLinkSpan, text string) *RenderLink

// RenderOptions 渲染选项。
// TS 对照: render.ts RenderOptions
type RenderOptions struct {
	StyleMarkers RenderStyleMap
	EscapeText   func(text string) string
	BuildLink    BuildLinkFn
}

// 样式渲染顺序（外层优先）。
// TS 对照: render.ts STYLE_ORDER
var styleOrder = []MarkdownStyle{
	StyleCodeBlock,
	StyleCode,
	StyleBold,
	StyleItalic,
	StyleStrikethrough,
	StyleSpoiler,
}

var styleRank map[MarkdownStyle]int

func init() {
	styleRank = make(map[MarkdownStyle]int)
	for i, s := range styleOrder {
		styleRank[s] = i
	}
}

// RenderMarkdownWithMarkers 将 IR 渲染为带样式标记的文本。
//
// 算法：
//  1. 收集所有样式和链接的 start/end 边界点
//  2. 在边界点处插入 open/close 标记
//  3. 使用 LIFO 栈确保嵌套正确
//
// TS 对照: render.ts renderMarkdownWithMarkers()
func RenderMarkdownWithMarkers(ir MarkdownIR, options RenderOptions) string {
	text := ir.Text
	if text == "" {
		return ""
	}

	escapeText := options.EscapeText
	if escapeText == nil {
		escapeText = func(s string) string { return s }
	}

	// 过滤有标记的样式 span
	var styled []MarkdownStyleSpan
	for _, span := range ir.Styles {
		if _, ok := options.StyleMarkers[span.Style]; ok {
			styled = append(styled, span)
		}
	}
	sortStyleSpans(styled)

	// 收集边界点
	boundaries := make(map[int]struct{})
	boundaries[0] = struct{}{}
	boundaries[len(text)] = struct{}{}

	// 按 start 位置分组
	startsAt := make(map[int][]MarkdownStyleSpan)
	for _, span := range styled {
		if span.Start == span.End {
			continue
		}
		boundaries[span.Start] = struct{}{}
		boundaries[span.End] = struct{}{}
		startsAt[span.Start] = append(startsAt[span.Start], span)
	}
	// 每个位置的 span 排序：end 降序，后按 rank
	for pos := range startsAt {
		spans := startsAt[pos]
		sort.Slice(spans, func(i, j int) bool {
			if spans[i].End != spans[j].End {
				return spans[j].End < spans[i].End
			}
			return styleRank[spans[i].Style] < styleRank[spans[j].Style]
		})
	}

	// 链接处理
	linkStarts := make(map[int][]RenderLink)
	if options.BuildLink != nil {
		for _, link := range ir.Links {
			if link.Start == link.End {
				continue
			}
			rendered := options.BuildLink(link, text)
			if rendered == nil {
				continue
			}
			boundaries[rendered.Start] = struct{}{}
			boundaries[rendered.End] = struct{}{}
			linkStarts[rendered.Start] = append(linkStarts[rendered.Start], *rendered)
		}
	}

	// 排序边界点
	points := make([]int, 0, len(boundaries))
	for p := range boundaries {
		points = append(points, p)
	}
	sort.Ints(points)

	// 渲染栈
	type stackEntry struct {
		close string
		end   int
	}
	var stack []stackEntry
	var out []byte

	for i, pos := range points {
		// 关闭到达边界的所有元素（LIFO）
		for len(stack) > 0 && stack[len(stack)-1].end == pos {
			out = append(out, stack[len(stack)-1].close...)
			stack = stack[:len(stack)-1]
		}

		// 收集此位置的 opening items
		type openItem struct {
			end   int
			open  string
			close string
			rank  int
		}
		var items []openItem

		if links, ok := linkStarts[pos]; ok {
			for _, lnk := range links {
				items = append(items, openItem{
					end:   lnk.End,
					open:  lnk.Open,
					close: lnk.Close,
					rank:  -1, // 链接优先
				})
			}
		}
		if spans, ok := startsAt[pos]; ok {
			for _, span := range spans {
				marker := options.StyleMarkers[span.Style]
				items = append(items, openItem{
					end:   span.End,
					open:  marker.Open,
					close: marker.Close,
					rank:  styleRank[span.Style],
				})
			}
		}

		// 排序: end 降序 → rank 升序
		sort.Slice(items, func(a, b int) bool {
			if items[a].end != items[b].end {
				return items[b].end < items[a].end
			}
			return items[a].rank < items[b].rank
		})

		for _, item := range items {
			out = append(out, item.open...)
			stack = append(stack, stackEntry{close: item.close, end: item.end})
		}

		// 输出此段文本
		if i+1 < len(points) {
			next := points[i+1]
			if next > pos && pos < len(text) {
				end := next
				if end > len(text) {
					end = len(text)
				}
				out = append(out, escapeText(text[pos:end])...)
			}
		}
	}

	return string(out)
}

// sortStyleSpans 按 start 升序、end 降序、rank 升序排列样式 span。
// TS 对照: render.ts sortStyleSpans()
func sortStyleSpans(spans []MarkdownStyleSpan) {
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start != spans[j].Start {
			return spans[i].Start < spans[j].Start
		}
		if spans[i].End != spans[j].End {
			return spans[j].End < spans[i].End
		}
		return styleRank[spans[i].Style] < styleRank[spans[j].Style]
	})
}
