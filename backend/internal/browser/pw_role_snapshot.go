// Package browser — Accessibility tree snapshot parser.
// TS source: pw-role-snapshot.ts (428L)
//
// Parses Playwright's aria/AI snapshots into a role-annotated tree with
// interactable element references (e.g. "e1", "e2"). These refs are used
// by the agent to address elements in click/fill/hover commands.
package browser

import (
	"fmt"
	"regexp"
	"strings"
)

// ---------- Types ----------

// RoleRef identifies a single interactable element in a snapshot.
type RoleRef struct {
	Role string `json:"role"`
	Name string `json:"name,omitempty"`
	// Nth is the disambiguation index when role+name duplicates exist.
	Nth *int `json:"nth,omitempty"`
}

// RoleRefMap maps ref ids (e.g. "e1") to their role info.
type RoleRefMap map[string]RoleRef

// RoleSnapshotStats summarises a snapshot.
type RoleSnapshotStats struct {
	Lines       int `json:"lines"`
	Chars       int `json:"chars"`
	Refs        int `json:"refs"`
	Interactive int `json:"interactive"`
}

// RoleSnapshotOptions controls snapshot generation.
type RoleSnapshotOptions struct {
	// Interactive filters to only interactive elements (buttons, links, inputs, etc.).
	Interactive bool
	// MaxDepth limits tree depth (0 = root only). Negative = unlimited.
	MaxDepth int
	// Compact removes unnamed structural elements and empty branches.
	Compact bool
}

// RoleSnapshotResult is the return value of BuildRoleSnapshot* functions.
type RoleSnapshotResult struct {
	Snapshot string     `json:"snapshot"`
	Refs     RoleRefMap `json:"refs"`
}

// ---------- Role sets ----------

var interactiveRoles = newStringSet(
	"button", "link", "textbox", "checkbox", "radio", "combobox",
	"listbox", "menuitem", "menuitemcheckbox", "menuitemradio",
	"option", "searchbox", "slider", "spinbutton", "switch", "tab", "treeitem",
)

var contentRoles = newStringSet(
	"heading", "cell", "gridcell", "columnheader", "rowheader",
	"listitem", "article", "region", "main", "navigation",
)

var structuralRoles = newStringSet(
	"generic", "group", "list", "table", "row", "rowgroup",
	"grid", "treegrid", "menu", "menubar", "toolbar", "tablist",
	"tree", "directory", "document", "application", "presentation", "none",
)

// ---------- Public functions ----------

// GetRoleSnapshotStats computes summary statistics for a snapshot.
func GetRoleSnapshotStats(snapshot string, refs RoleRefMap) RoleSnapshotStats {
	interactive := 0
	for _, r := range refs {
		if interactiveRoles.has(r.Role) {
			interactive++
		}
	}
	lines := 1
	for _, c := range snapshot {
		if c == '\n' {
			lines++
		}
	}
	return RoleSnapshotStats{
		Lines:       lines,
		Chars:       len(snapshot),
		Refs:        len(refs),
		Interactive: interactive,
	}
}

// ParseRoleRef normalizes a raw ref string ("@e12", "ref=e12", "e12") → "e12".
// Returns empty string if invalid.
func ParseRoleRef(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalized := trimmed
	if strings.HasPrefix(normalized, "@") {
		normalized = normalized[1:]
	} else if strings.HasPrefix(normalized, "ref=") {
		normalized = normalized[4:]
	}
	if refPattern.MatchString(normalized) {
		return normalized
	}
	return ""
}

// BuildRoleSnapshotFromAriaSnapshot builds a role snapshot with generated refs
// from a raw Playwright ariaSnapshot() output.
func BuildRoleSnapshotFromAriaSnapshot(ariaSnapshot string, options RoleSnapshotOptions) RoleSnapshotResult {
	lines := strings.Split(ariaSnapshot, "\n")
	refs := make(RoleRefMap)
	tracker := newRoleNameTracker()

	counter := 0
	nextRef := func() string {
		counter++
		return fmt.Sprintf("e%d", counter)
	}

	if options.Interactive {
		var result []string
		for _, line := range lines {
			depth := getIndentLevel(line)
			if options.MaxDepth > 0 && depth > options.MaxDepth {
				continue
			}

			match := linePattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			roleRaw := match[2]
			name := match[3]
			suffix := match[4]

			if strings.HasPrefix(roleRaw, "/") {
				continue
			}

			role := strings.ToLower(roleRaw)
			if !interactiveRoles.has(role) {
				continue
			}

			ref := nextRef()
			nth := tracker.getNextIndex(role, name)
			tracker.trackRef(role, name, ref)
			r := RoleRef{Role: role}
			if name != "" {
				r.Name = name
			}
			nthVal := nth
			r.Nth = &nthVal
			refs[ref] = r

			enhanced := "- " + roleRaw
			if name != "" {
				enhanced += fmt.Sprintf(` "%s"`, name)
			}
			enhanced += " [ref=" + ref + "]"
			if nth > 0 {
				enhanced += fmt.Sprintf(" [nth=%d]", nth)
			}
			if strings.Contains(suffix, "[") {
				enhanced += suffix
			}
			result = append(result, enhanced)
		}

		removeNthFromNonDuplicates(refs, tracker)

		snap := strings.Join(result, "\n")
		if snap == "" {
			snap = "(no interactive elements)"
		}
		return RoleSnapshotResult{Snapshot: snap, Refs: refs}
	}

	// Full mode
	var result []string
	for _, line := range lines {
		processed := processLine(line, refs, options, tracker, nextRef)
		if processed != nil {
			result = append(result, *processed)
		}
	}

	removeNthFromNonDuplicates(refs, tracker)

	tree := strings.Join(result, "\n")
	if tree == "" {
		tree = "(empty)"
	}
	if options.Compact {
		tree = compactTree(tree)
	}
	return RoleSnapshotResult{Snapshot: tree, Refs: refs}
}

// BuildRoleSnapshotFromAiSnapshot builds a role snapshot from Playwright's AI
// snapshot output while preserving Playwright's own aria-ref ids (e.g. ref=e13).
func BuildRoleSnapshotFromAiSnapshot(aiSnapshot string, options RoleSnapshotOptions) RoleSnapshotResult {
	raw := aiSnapshot
	if raw == "" {
		raw = ""
	}
	lines := strings.Split(raw, "\n")
	refs := make(RoleRefMap)

	if options.Interactive {
		var out []string
		for _, line := range lines {
			depth := getIndentLevel(line)
			if options.MaxDepth > 0 && depth > options.MaxDepth {
				continue
			}
			match := linePattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			roleRaw := match[2]
			name := match[3]
			suffix := match[4]

			if strings.HasPrefix(roleRaw, "/") {
				continue
			}
			role := strings.ToLower(roleRaw)
			if !interactiveRoles.has(role) {
				continue
			}
			ref := parseAiSnapshotRef(suffix)
			if ref == "" {
				continue
			}
			r := RoleRef{Role: role}
			if name != "" {
				r.Name = name
			}
			refs[ref] = r
			enhanced := "- " + roleRaw
			if name != "" {
				enhanced += fmt.Sprintf(` "%s"`, name)
			}
			enhanced += suffix
			out = append(out, enhanced)
		}
		snap := strings.Join(out, "\n")
		if snap == "" {
			snap = "(no interactive elements)"
		}
		return RoleSnapshotResult{Snapshot: snap, Refs: refs}
	}

	// Full mode
	var out []string
	for _, line := range lines {
		depth := getIndentLevel(line)
		if options.MaxDepth > 0 && depth > options.MaxDepth {
			continue
		}

		match := linePattern.FindStringSubmatch(line)
		if match == nil {
			out = append(out, line)
			continue
		}
		roleRaw := match[2]
		name := match[3]
		suffix := match[4]

		if strings.HasPrefix(roleRaw, "/") {
			out = append(out, line)
			continue
		}

		role := strings.ToLower(roleRaw)
		isStructural := structuralRoles.has(role)
		if options.Compact && isStructural && name == "" {
			continue
		}

		ref := parseAiSnapshotRef(suffix)
		if ref != "" {
			r := RoleRef{Role: role}
			if name != "" {
				r.Name = name
			}
			refs[ref] = r
		}

		out = append(out, line)
	}

	tree := strings.Join(out, "\n")
	if tree == "" {
		tree = "(empty)"
	}
	if options.Compact {
		tree = compactTree(tree)
	}
	return RoleSnapshotResult{Snapshot: tree, Refs: refs}
}

// ---------- Internal helpers ----------

var (
	// linePattern matches "  - roleName \"label\" [rest]"
	linePattern = regexp.MustCompile(`^(\s*-\s*)(\w+)(?:\s+"([^"]*)")?(.*)$`)
	// refPattern matches "e123"
	refPattern = regexp.MustCompile(`^e\d+$`)
	// aiRefPattern extracts ref=e13 from a suffix string
	aiRefPattern = regexp.MustCompile(`\[ref=(e\d+)\]`)
	// indentPattern matches leading whitespace
	indentPattern = regexp.MustCompile(`^(\s*)`)
)

func getIndentLevel(line string) int {
	match := indentPattern.FindStringSubmatch(line)
	if match == nil {
		return 0
	}
	return len(match[1]) / 2
}

func parseAiSnapshotRef(suffix string) string {
	match := aiRefPattern.FindStringSubmatch(suffix)
	if match != nil {
		return match[1]
	}
	return ""
}

// roleNameTracker deduplicates role+name combinations and tracks refs.
type roleNameTracker struct {
	counts    map[string]int
	refsByKey map[string][]string
}

func newRoleNameTracker() *roleNameTracker {
	return &roleNameTracker{
		counts:    make(map[string]int),
		refsByKey: make(map[string][]string),
	}
}

func (t *roleNameTracker) getKey(role, name string) string {
	return role + ":" + name
}

func (t *roleNameTracker) getNextIndex(role, name string) int {
	key := t.getKey(role, name)
	current := t.counts[key]
	t.counts[key] = current + 1
	return current
}

func (t *roleNameTracker) trackRef(role, name, ref string) {
	key := t.getKey(role, name)
	t.refsByKey[key] = append(t.refsByKey[key], ref)
}

func (t *roleNameTracker) getDuplicateKeys() map[string]bool {
	out := make(map[string]bool)
	for key, refs := range t.refsByKey {
		if len(refs) > 1 {
			out[key] = true
		}
	}
	return out
}

func removeNthFromNonDuplicates(refs RoleRefMap, tracker *roleNameTracker) {
	duplicates := tracker.getDuplicateKeys()
	for ref, data := range refs {
		key := tracker.getKey(data.Role, data.Name)
		if !duplicates[key] {
			data.Nth = nil
			refs[ref] = data
		}
	}
}

func processLine(
	line string,
	refs RoleRefMap,
	options RoleSnapshotOptions,
	tracker *roleNameTracker,
	nextRef func() string,
) *string {
	depth := getIndentLevel(line)
	if options.MaxDepth > 0 && depth > options.MaxDepth {
		return nil
	}

	match := linePattern.FindStringSubmatch(line)
	if match == nil {
		if options.Interactive {
			return nil
		}
		return &line
	}

	prefix := match[1]
	roleRaw := match[2]
	name := match[3]
	suffix := match[4]

	if strings.HasPrefix(roleRaw, "/") {
		if options.Interactive {
			return nil
		}
		return &line
	}

	role := strings.ToLower(roleRaw)
	isInteractive := interactiveRoles.has(role)
	isContent := contentRoles.has(role)
	isStructural := structuralRoles.has(role)

	if options.Interactive && !isInteractive {
		return nil
	}
	if options.Compact && isStructural && name == "" {
		return nil
	}

	shouldHaveRef := isInteractive || (isContent && name != "")
	if !shouldHaveRef {
		return &line
	}

	ref := nextRef()
	nth := tracker.getNextIndex(role, name)
	tracker.trackRef(role, name, ref)
	r := RoleRef{Role: role}
	if name != "" {
		r.Name = name
	}
	nthVal := nth
	r.Nth = &nthVal
	refs[ref] = r

	enhanced := prefix + roleRaw
	if name != "" {
		enhanced += fmt.Sprintf(` "%s"`, name)
	}
	enhanced += " [ref=" + ref + "]"
	if nth > 0 {
		enhanced += fmt.Sprintf(" [nth=%d]", nth)
	}
	if suffix != "" {
		enhanced += suffix
	}
	return &enhanced
}

func compactTree(tree string) string {
	lines := strings.Split(tree, "\n")
	var result []string

	for i, line := range lines {
		if strings.Contains(line, "[ref=") {
			result = append(result, line)
			continue
		}
		if strings.Contains(line, ":") && !strings.HasSuffix(strings.TrimSpace(line), ":") {
			result = append(result, line)
			continue
		}

		currentIndent := getIndentLevel(line)
		hasRelevantChildren := false
		for j := i + 1; j < len(lines); j++ {
			childIndent := getIndentLevel(lines[j])
			if childIndent <= currentIndent {
				break
			}
			if strings.Contains(lines[j], "[ref=") {
				hasRelevantChildren = true
				break
			}
		}
		if hasRelevantChildren {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// stringSet is a simple set backed by a map.
type stringSet map[string]struct{}

func newStringSet(values ...string) stringSet {
	s := make(stringSet, len(values))
	for _, v := range values {
		s[v] = struct{}{}
	}
	return s
}

func (s stringSet) has(value string) bool {
	_, ok := s[value]
	return ok
}
