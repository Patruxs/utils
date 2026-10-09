package views

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	corenetwork "utils/internal/core/network"
	"utils/internal/ui/common"
)

type networkCardMode int

const (
	networkCardFull networkCardMode = iota
	networkCardLine
	networkCardHidden
)

const (
	networkWideColumns    = 100
	networkDetailColumns  = 80
	networkCardMinRows    = 24
	networkCardRows       = 4
	networkActionsShare   = 36
	networkActionsMin     = 44
	networkActionsMax     = 48
	networkStackedMinList = 5
	networkColumnWidth    = 12
	networkCheckWidth     = 4
	networkInfoMaxWidth   = 64
	networkStatusLabel    = 8
	networkDNSShown       = 2
	networkUnknown        = "?"
	networkCursor         = "▸ "
	networkWritesTag      = "writes"
	networkSectionGlyph   = "◆"
	networkModalWidth     = 68
	networkModalChrome    = 6
)

var networkUUID = regexp.MustCompile(`[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}`)

type networkLayout struct {
	width        int
	height       int
	card         networkCardMode
	stacked      bool
	detailHidden bool
	actionsWidth int
	detailWidth  int
}

type networkRow struct {
	group string
	items []networkActionID
}

func (m NetworkModel) bodySize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = common.DefaultContentWidth - 2*common.MarginX
	}
	if height <= 0 {
		height = common.DefaultContentHeight - common.HeaderHeight(common.DefaultContentHeight) - 1
	}
	return width, height
}

func (m NetworkModel) layout() networkLayout {
	width, height := m.bodySize()
	columns := width + 2*common.MarginX
	rows := height + common.HeaderHeight(height) + 1
	l := networkLayout{width: width, height: height, stacked: columns < networkWideColumns, detailHidden: columns < networkDetailColumns}
	switch {
	case rows < networkCardMinRows:
		l.card = networkCardHidden
	case l.stacked:
		l.card = networkCardLine
	}
	l.actionsWidth, l.detailWidth = width, width
	if !l.stacked {
		l.actionsWidth = common.MinInt(networkActionsMax, common.MaxInt(networkActionsMin, width*networkActionsShare/100))
		l.detailWidth = width - l.actionsWidth - 1
	}
	return l
}

func (l networkLayout) cardHeight() int {
	switch l.card {
	case networkCardFull:
		return networkCardRows
	case networkCardLine:
		return 1
	default:
		return 0
	}
}

func (m NetworkModel) detailTakesBody() bool {
	return m.state != networkStateSelectingOptions
}

func (m NetworkModel) paneHeights() (int, int) {
	l := m.layout()
	area := common.MaxInt(3, l.height-l.cardHeight())
	if !l.stacked {
		return area, area
	}
	if m.detailTakesBody() {
		return 0, area
	}
	if l.detailHidden {
		return area, 0
	}
	listNeed := len(networkRows(false)) + 2
	detailNeed := len(m.detailLines(l.detailWidth-4)) + 2
	if listNeed+detailNeed <= area {
		return area - detailNeed, detailNeed
	}
	detail := common.MinInt(detailNeed, area/2)
	return common.MaxInt(networkStackedMinList, area-detail), common.MaxInt(0, area-common.MaxInt(networkStackedMinList, area-detail))
}

func (m NetworkModel) listRowCapacity() int {
	actions, _ := m.paneHeights()
	return common.MaxInt(1, actions-2)
}

func (m NetworkModel) expanded() bool {
	return len(networkRows(true)) <= m.listRowCapacity()
}

func networkRows(expanded bool) []networkRow {
	var rows []networkRow
	group := ""
	for _, item := range networkActions {
		if item.group != group {
			group = item.group
			rows = append(rows, networkRow{group: group})
			if !expanded && isInlineGroup(group) {
				rows = append(rows, networkRow{group: group})
			}
		}
		if !expanded && isInlineGroup(group) {
			last := &rows[len(rows)-1]
			last.items = append(last.items, item.id)
			continue
		}
		rows = append(rows, networkRow{group: group, items: []networkActionID{item.id}})
	}
	return rows
}

func (r networkRow) header() bool {
	return len(r.items) == 0
}

func (r networkRow) inline() bool {
	return len(r.items) > 1
}

func (r networkRow) selectable() bool {
	for _, id := range r.items {
		if networkItem(id).supported() {
			return true
		}
	}
	return false
}

func (m NetworkModel) rows() []networkRow {
	return networkRows(m.expanded())
}

func (m NetworkModel) focusRow(rows []networkRow) int {
	for index, row := range rows {
		for _, id := range row.items {
			if id == m.focus {
				return index
			}
		}
	}
	return 0
}

func (m NetworkModel) focusedRowIsInline() bool {
	rows := m.rows()
	return rows[m.focusRow(rows)].inline()
}

func (m *NetworkModel) moveFocus(delta int) {
	rows := m.rows()
	for index := m.focusRow(rows) + delta; index >= 0 && index < len(rows); index += delta {
		if rows[index].header() || !rows[index].selectable() {
			continue
		}
		m.setFocus(m.rowChoice(rows[index]))
		break
	}
	m.keepFocusVisible()
}

func (m NetworkModel) rowChoice(row networkRow) networkActionID {
	if row.inline() {
		if slot := inlineSlot(row.group); slot >= 0 && networkItem(m.choices[slot]).supported() {
			return m.choices[slot]
		}
	}
	for _, id := range row.items {
		if networkItem(id).supported() {
			return id
		}
	}
	return row.items[0]
}

func (m *NetworkModel) setFocus(id networkActionID) {
	m.focus = id
	if slot := inlineSlot(networkItem(id).group); slot >= 0 {
		m.choices[slot] = id
	}
}

func (m *NetworkModel) chooseInline(left bool) {
	rows := m.rows()
	row := rows[m.focusRow(rows)]
	if !row.inline() {
		return
	}
	position := 0
	for index, id := range row.items {
		if id == m.focus {
			position = index
		}
	}
	step := 1
	if left {
		step = -1
	}
	for index := position + step; index >= 0 && index < len(row.items); index += step {
		if networkItem(row.items[index]).supported() {
			m.setFocus(row.items[index])
			return
		}
	}
}

func inlineSlot(group string) int {
	for index, inline := range networkInlineGroups {
		if inline == group {
			return index
		}
	}
	return -1
}

func (m *NetworkModel) keepFocusVisible() {
	rows := m.rows()
	capacity := m.listRowCapacity()
	if len(rows) <= capacity {
		m.listOffset = 0
		return
	}
	focus := m.focusRow(rows)
	if focus > 0 && rows[focus-1].header() {
		focus--
	}
	maxOffset := len(rows) - capacity
	for range 3 {
		top, bottom := 0, 0
		if m.listOffset > 0 {
			top = 1
		}
		if m.listOffset+capacity < len(rows) {
			bottom = 1
		}
		switch {
		case focus < m.listOffset+top:
			m.listOffset = focus - 1
		case m.focusRow(rows) > m.listOffset+capacity-1-bottom:
			m.listOffset = m.focusRow(rows) - capacity + 2
		}
		m.listOffset = common.MinInt(maxOffset, common.MaxInt(0, m.listOffset))
	}
}

func (m *NetworkModel) layoutComponents() {
	l := m.layout()
	_, detailHeight := m.paneHeights()
	m.logViewer.SetSize(common.MaxInt(1, l.detailWidth-4), common.MaxInt(1, detailHeight-2))
	inputWidth := common.MaxInt(10, l.detailWidth-18)
	m.hostDomainInput.Width = inputWidth
	m.hostIPInput.Width = inputWidth
}

func (m NetworkModel) View() string {
	l := m.layout()
	var parts []string
	switch l.card {
	case networkCardFull:
		parts = append(parts, m.statusCard(l.width))
	case networkCardLine:
		parts = append(parts, common.Truncate(m.statusLine(), l.width))
	}

	actionsHeight, detailHeight := m.paneHeights()
	var actions, detail string
	if actionsHeight > 0 {
		actions = m.actionsPane(l.actionsWidth, actionsHeight)
	}
	if detailHeight > 0 {
		detail = m.detailPane(l.detailWidth, detailHeight)
	}
	switch {
	case !l.stacked:
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, actions, " ", detail))
	case actions != "" && detail != "":
		parts = append(parts, actions, detail)
	case actions != "":
		parts = append(parts, actions)
	default:
		parts = append(parts, detail)
	}
	return strings.Join(parts, "\n")
}

func (m NetworkModel) statusCard(width int) string {
	meta := ""
	switch {
	case m.statusLoading && !m.statusLoaded:
		meta = "loading…"
	case m.statusLoading:
		meta = "refreshing…"
	case m.statusLoaded:
		meta = "refreshed " + common.FormatElapsed(time.Since(m.statusAt))
	}
	panel := common.Panel{Title: "Status", Meta: meta, Width: width}
	inner := panel.InnerWidth()
	if !m.statusLoaded {
		return panel.Render(common.Muted.Render("loading…") + "\n")
	}

	s := m.status
	left := common.PadRight(orUnknown(s.Adapter), networkStatusLabel) + " " + orUnknown(s.Address) + common.Muted.Render(" · ") + "gw " + orUnknown(s.Gateway) + common.Muted.Render(" · ") + "MTU " + orUnknownInt(s.MTU)
	first := common.SpreadLine(inner, left, m.fittedDNS(inner-lipgloss.Width(left)-2))

	doh := common.PadRight("DoH", networkStatusLabel) + " " + dohText(s.DoH)
	persistent := "Persistent DNS " + persistentText(s)
	second := common.SpreadLine(inner, common.PadRight(doh, inner*2/5)+persistent, "sudo  "+sudoText(s.Sudo))
	return panel.Render(first + "\n" + second)
}

func (m NetworkModel) fittedDNS(room int) string {
	source := ""
	if m.status.DNSSource != "" {
		source = common.Muted.Render(" via " + m.status.DNSSource)
	}
	candidates := []string{
		"DNS " + m.dnsServers(networkDNSShown) + source,
		"DNS " + m.dnsServers(1) + source,
		"DNS " + m.dnsServers(1),
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= room {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func (m NetworkModel) dnsServers(limit int) string {
	servers := m.status.DNS
	if len(servers) == 0 {
		return networkUnknown
	}
	shown := strings.Join(servers[:common.MinInt(limit, len(servers))], ", ")
	if len(servers) > limit {
		shown += common.Muted.Render(fmt.Sprintf(" +%d", len(servers)-limit))
	}
	return shown
}

func (m NetworkModel) statusLine() string {
	if !m.statusLoaded {
		return common.Muted.Render("Status loading…")
	}
	return m.statusSummary()
}

func (m NetworkModel) statusSummary() string {
	s := m.status
	address, _, _ := strings.Cut(s.Address, "/")
	return fmt.Sprintf("%s %s · DNS %s · sudo %s", orUnknown(s.Adapter), orUnknown(address), m.dnsServers(1), sudoGlyph(s.Sudo))
}

func orUnknown(value string) string {
	if value == "" {
		return networkUnknown
	}
	return value
}

func orUnknownInt(value int) string {
	if value <= 0 {
		return networkUnknown
	}
	return fmt.Sprint(value)
}

func dohText(state corenetwork.State) string {
	switch state {
	case corenetwork.StateOn:
		return "on"
	case corenetwork.StateOff:
		return "off"
	case corenetwork.StateUnsupported:
		return common.Muted.Render("not available on " + runtime.GOOS)
	default:
		return networkUnknown
	}
}

func persistentText(s corenetwork.Status) string {
	switch s.Persistent {
	case corenetwork.StateOn:
		if s.Saved.DNSName != "" {
			return "on · " + s.Saved.DNSName
		}
		return "on"
	case corenetwork.StateOff:
		return "off"
	default:
		return networkUnknown
	}
}

func sudoText(state corenetwork.State) string {
	switch state {
	case corenetwork.StateOn:
		return "cached"
	case corenetwork.StateOff:
		return common.Warning.Render("not cached · run sudo -v")
	case corenetwork.StateUnsupported:
		return "UAC prompt"
	default:
		return networkUnknown
	}
}

func sudoGlyph(state corenetwork.State) string {
	switch state {
	case corenetwork.StateOn:
		return common.ToneSuccess.Glyph()
	case corenetwork.StateOff:
		return common.ToneDanger.Glyph()
	case corenetwork.StateUnsupported:
		return "UAC"
	default:
		return networkUnknown
	}
}

func (m NetworkModel) actionsPane(width, height int) string {
	variant := common.PanelFocused
	if m.state != networkStateSelectingOptions {
		variant = common.PanelNormal
	}
	panel := common.Panel{Title: "Actions", Variant: variant, Width: width, Height: height}
	inner := panel.InnerWidth()
	rows := m.rows()
	capacity := common.MaxInt(1, height-2)
	offset := common.MinInt(m.listOffset, common.MaxInt(0, len(rows)-capacity))
	end := common.MinInt(len(rows), offset+capacity)

	lines := make([]string, 0, capacity)
	for index := offset; index < end; index++ {
		lines = append(lines, m.renderRow(rows[index], inner))
	}
	if offset > 0 {
		lines[0] = common.Muted.Render(fmt.Sprintf("  ↑ %d more", offset))
	}
	if end < len(rows) {
		lines[len(lines)-1] = common.Muted.Render(fmt.Sprintf("  ↓ %d more", len(rows)-end+1))
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m NetworkModel) renderRow(row networkRow, width int) string {
	if row.header() {
		return common.Muted.Bold(true).Render(row.group)
	}
	focused := m.focusRowIs(row)
	cursor := "  "
	if focused && m.state == networkStateSelectingOptions {
		cursor = common.Accent.Render(networkCursor)
	}
	check := ""
	if len(m.batch) > 0 {
		check = "[ ] "
		for _, id := range row.items {
			if m.inBatch(id) {
				check = "[x] "
			}
		}
	}
	if row.inline() {
		return cursor + check + m.renderChoices(row, width-lipgloss.Width(cursor+check), focused)
	}

	item := networkItem(row.items[0])
	name := item.name
	if item.column != "" {
		name = common.PadRight(name, networkColumnWidth) + item.column
	}
	tag := ""
	switch {
	case !item.supported():
		return cursor + check + common.Muted.Render(common.SpreadLine(width-lipgloss.Width(cursor+check), name, "windows only"))
	case item.writes():
		tag = common.Muted.Render(networkWritesTag)
	}
	if focused {
		name = common.Selected.Render(name)
	}
	return common.SpreadLine(width, cursor+check+name, tag)
}

func (m NetworkModel) focusRowIs(row networkRow) bool {
	for _, id := range row.items {
		if id == m.focus {
			return true
		}
	}
	return false
}

func (m NetworkModel) renderChoices(row networkRow, width int, focused bool) string {
	separator := common.Muted.Render(" · ")
	tokens := make([]string, len(row.items))
	selected := 0
	for index, id := range row.items {
		item := networkItem(id)
		label := item.name
		if m.inBatch(id) {
			label += "✓"
		}
		switch {
		case !item.supported():
			label = common.Muted.Render(label)
		case focused && id == m.focus:
			label = common.Selected.Underline(true).Render("‹" + label + "›")
			selected = index
		}
		tokens[index] = label
	}
	first, last := 0, len(tokens)
	joined := func() string {
		text := strings.Join(tokens[first:last], separator)
		if first > 0 {
			text = common.Muted.Render("… ") + text
		}
		if last < len(tokens) {
			text += common.Muted.Render(" …")
		}
		return text
	}
	for lipgloss.Width(joined()) > width && last-first > 1 {
		if selected-first > last-1-selected {
			first++
		} else {
			last--
		}
	}
	return common.Truncate(joined(), width)
}

func (m NetworkModel) detailPane(width, height int) string {
	switch m.state {
	case networkStateRunning:
		return m.runningPane(width, height)
	case networkStateFinished:
		return m.resultPane(width, height)
	case networkStateEditingHostsAdd:
		panel := common.Panel{Title: actionTitle(networkActionHostsAdd), Variant: common.PanelFocused, Width: width}
		if m.layout().stacked {
			panel.Height = height
		}
		return panel.Render(strings.Join(m.hostsFormLines(panel.InnerWidth()), "\n"))
	}
	panel := common.Panel{Title: networkItem(m.focus).title, Width: width}
	if m.layout().stacked {
		panel.Height = height
	}
	return panel.Render(strings.Join(m.detailLines(panel.InnerWidth()), "\n"))
}

func (m NetworkModel) infoOverlay() string {
	width, _ := m.bodySize()
	panel := common.Panel{Title: networkItem(m.focus).title, Variant: common.PanelFocused, Width: common.MinInt(width-4, networkInfoMaxWidth)}
	lines := append(m.detailLines(panel.InnerWidth()), "", common.Muted.Render("i or esc close"))
	return panel.Render(strings.Join(lines, "\n"))
}

func (m NetworkModel) detailLines(width int) []string {
	id := m.focus
	item := networkItem(id)
	lines := []string{tagLine(item), ""}
	for _, line := range common.WrapPlain(m.actionSummary(id), width) {
		lines = append(lines, line)
	}
	lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Runs"))
	switch commands := m.previewLines(id); {
	case !item.supported():
		lines = append(lines, common.Muted.Render("  not available on "+runtime.GOOS))
	case len(commands) == 0:
		lines = append(lines, common.Muted.Render("  nothing to run on this system"))
	default:
		for _, command := range commands {
			lines = append(lines, common.Truncate("  "+command, width))
		}
	}
	if item.undo != networkNoUndo {
		lines = append(lines, "", common.Muted.Render("Undo: ")+networkItem(item.undo).title)
	}
	return lines
}

func tagLine(item networkActionItem) string {
	switch {
	case !item.supported():
		return common.Muted.Render("windows only")
	case !item.writes():
		return common.Muted.Render("read-only")
	}
	dot := common.Warning.Render("●")
	if !item.action.Elevated() {
		return dot + " writes · current user"
	}
	if runtime.GOOS == osWindows {
		return dot + " writes · elevated via UAC"
	}
	return dot + " writes · elevated via sudo"
}

func (m NetworkModel) previewLines(id networkActionID) []string {
	return m.previewWith(id, m.params(id))
}

func (m NetworkModel) previewWith(id networkActionID, params corenetwork.Params) []string {
	lines := corenetwork.Preview(networkItem(id).action, params, m.status)
	for index, line := range lines {
		lines[index] = networkUUID.ReplaceAllStringFunc(line, func(uuid string) string { return uuid[:8] + "…" })
	}
	return lines
}

func (m NetworkModel) hostsFormLines(width int) []string {
	domainErr, ipErr := m.hostsFieldErrors()
	lines := []string{common.Warning.Render("●") + " writes " + corenetwork.HostsPath() + " · elevated", ""}
	field := func(index int, label string, input string, problem string) {
		cursor := "  "
		if m.focusedHostField == index {
			cursor = common.Accent.Render(networkCursor)
			label = lipgloss.NewStyle().Bold(true).Render(label)
		}
		lines = append(lines, cursor+label+common.Accent.Render("▏")+input)
		if problem != "" {
			for _, line := range common.WrapLine("          "+common.ToneDanger.Glyph()+" ", problem, width) {
				lines = append(lines, common.Error.Render(line))
			}
		}
	}
	field(0, "Domain  ", m.hostDomainInput.View(), domainErr)
	field(1, "IP      ", m.hostIPInput.View(), ipErr)
	lines = append(lines, "")
	if domainErr == "" && ipErr == "" {
		entry := m.hostsEntry()
		lines = append(lines, common.Muted.Render("Appends  ")+entry.IP+"  "+entry.Domain+"  "+common.Muted.Render(corenetwork.HostsManagedMarker))
	} else {
		lines = append(lines, common.Muted.Render("Appends  nothing until both fields are valid"))
	}
	return append(lines, "", common.Muted.Render("tab next · enter add · esc back"))
}

func (m NetworkModel) runningPane(width, height int) string {
	id := m.focus
	if m.runIndex < len(m.running) {
		id = m.running[m.runIndex]
	}
	title := m.spinner.View() + " " + actionTitle(id)
	if len(m.running) > 1 {
		title = fmt.Sprintf("%s %d/%d %s", m.spinner.View(), m.runIndex+1, len(m.running), actionTitle(id))
	}
	title += " · " + formatTenths(time.Since(m.runStarted))
	panel := common.Panel{Title: title, Variant: common.PanelFocused, Width: width, Height: height}
	rows := common.MaxInt(1, height-2)
	output := m.output
	if len(output) > rows {
		output = output[len(output)-rows:]
	}
	if len(output) == 0 {
		return panel.Render(common.Muted.Render("waiting for output…"))
	}
	return panel.Render(strings.Join(output, "\n"))
}

func (m NetworkModel) resultPane(width, height int) string {
	panel := common.Panel{Title: m.resultTitle(), Meta: m.logViewer.Position(), Variant: common.PanelFocused, Width: width, Height: height}
	log := m.logViewer.View()
	if log == "" {
		log = common.Muted.Render("No output")
	}
	return panel.Render(log)
}

func (m NetworkModel) resultTitle() string {
	name := fmt.Sprintf("%d actions", len(m.results))
	if len(m.results) == 1 {
		name = actionTitle(m.results[0].action)
	}
	failed := 0
	for _, result := range m.results {
		if result.err != nil && !errors.Is(result.err, context.Canceled) {
			failed++
		}
	}
	switch {
	case m.canceled:
		return common.Warning.Render(common.ToneWarning.Glyph() + " Canceled · " + name)
	case failed > 0 && len(m.results) > 1:
		return common.Error.Render(fmt.Sprintf("%s %d of %s failed", common.ToneDanger.Glyph(), failed, name))
	case failed > 0:
		return common.Error.Render(common.ToneDanger.Glyph() + " " + name + " failed")
	}
	return common.Success.Render(common.ToneSuccess.Glyph()) + " " + name + common.Muted.Render(" · "+formatTenths(m.runElapsed))
}

func (m NetworkModel) confirmLines(options networkRunOptions) []string {
	var lines []string
	elevated := false
	for _, id := range options.actions {
		item := networkItem(id)
		if !item.writes() {
			continue
		}
		elevated = elevated || item.action.Elevated()
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(item.title))
		width, _ := m.bodySize()
		for _, command := range m.previewWith(id, options.params[id]) {
			wrapped := common.WrapLine("    ", command, common.MinInt(networkModalWidth, width)-networkModalChrome)
			wrapped[0] = "  " + strings.TrimPrefix(wrapped[0], "    ")
			lines = append(lines, wrapped...)
		}
	}
	return append(lines, "", m.elevationLine(elevated))
}

func (m NetworkModel) elevationLine(elevated bool) string {
	switch {
	case !elevated:
		return "Runs as the current user."
	case runtime.GOOS == osWindows:
		return "Opens a Windows UAC prompt."
	}
	switch m.status.Sudo {
	case corenetwork.StateOn:
		return "Needs sudo. Cached: yes."
	case corenetwork.StateOff:
		return common.Warning.Render(common.ToneWarning.Glyph() + " Needs sudo. Cached: no. Run sudo -v first.")
	default:
		return common.Warning.Render(common.ToneWarning.Glyph() + " Needs sudo. Cached: unknown. Run sudo -v first.")
	}
}

func networkLogSections(results []networkActionResult) []common.LogSection {
	var problems []common.LogLine
	sections := make([]common.LogSection, 0, len(results)+1)
	for index, result := range results {
		section := common.LogSection{ID: fmt.Sprintf("action-%d", index), Title: actionTitle(result.action), Glyph: networkSectionGlyph, Tone: common.ToneAccent}
		if len(results) > 1 {
			section.Glyph, section.Tone = common.ToneSuccess.Glyph(), common.ToneSuccess
			if result.err != nil {
				section.Glyph, section.Tone = common.ToneDanger.Glyph(), common.ToneDanger
			}
		}
		for _, line := range networkResultLines(result) {
			section.Lines = append(section.Lines, line)
			if line.Tone == common.ToneWarning || line.Tone == common.ToneDanger {
				if len(results) > 1 {
					line.Detail = actionTitle(result.action)
				}
				problems = append(problems, line)
			}
		}
		sections = append(sections, section)
	}
	if len(results) == 1 {
		sections[0].Title = ""
		if len(problems) > 0 {
			sections[0].Title, sections[0].Glyph, sections[0].Tone = "OUTPUT", "", common.ToneNormal
		}
	}
	if len(problems) == 0 {
		return sections
	}

	problemTone := common.ToneWarning
	for _, line := range problems {
		if line.Tone == common.ToneDanger {
			problemTone = common.ToneDanger
		}
	}
	return append([]common.LogSection{{ID: "problems", Title: "PROBLEMS", Tone: problemTone, Lines: problems}}, sections...)
}

func networkResultLines(result networkActionResult) []common.LogLine {
	lines := make([]common.LogLine, 0, len(result.report.Entries)+1)
	for _, entry := range result.report.Entries {
		lines = append(lines, common.LogLine{Tone: networkLevelTone(entry.Level), Text: entry.Message})
	}
	if result.err != nil && result.report.Errors == 0 && !errors.Is(result.err, context.Canceled) {
		lines = append(lines, common.LogLine{Tone: common.ToneDanger, Text: fmt.Sprintf("%s failed: %v", actionTitle(result.action), result.err)})
	}
	return lines
}

func networkProblemCounts(results []networkActionResult) (int, int) {
	warnings, errorCount := 0, 0
	for _, result := range results {
		for _, line := range networkResultLines(result) {
			switch line.Tone {
			case common.ToneWarning:
				warnings++
			case common.ToneDanger:
				errorCount++
			}
		}
	}
	return warnings, errorCount
}

func networkLevelTone(level corenetwork.Level) common.Tone {
	switch level {
	case corenetwork.LevelWarn:
		return common.ToneWarning
	case corenetwork.LevelError:
		return common.ToneDanger
	case corenetwork.LevelSuccess:
		return common.ToneSuccess
	default:
		return common.ToneNormal
	}
}
