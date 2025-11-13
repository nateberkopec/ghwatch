package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/nateberkopec/ghwatch/internal/githubclient"
	"github.com/nateberkopec/ghwatch/internal/watch"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("247"))

	rowStyle = lipgloss.NewStyle()

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("230"))

	statusNeutralStyle = lipgloss.NewStyle()
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	statusSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	inputStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	inputFocusedStyle = inputStyle.Copy().BorderForeground(lipgloss.Color("105"))

	tableGap = " │ "
)

var tableColumns = []struct {
	Title  string
	Weight float64
	Min    int
}{
	{"", 0.05, 2},
	{"Repo", 0.21, 14},
	{"Owner", 0.15, 10},
	{"Target", 0.18, 12},
	{"Run", 0.20, 16},
	{"Workflow", 0.21, 12},
}

func renderView(m *Model) string {
	if m.width == 0 || m.height == 0 {
		return "Loading…"
	}

	var out []string
	out = append(out, renderInputField(m))
	out = append(out, renderHelpText(m))
	out = append(out, renderRunsTable(m))
	out = append(out, renderStatusLine(m))

	return strings.Join(out, "\n")
}

func renderHeader(m *Model) string {
	mode := "active"
	if m.showArchived {
		mode = "archived"
	}
	text := fmt.Sprintf("filter: %s • bell: %s", mode, bellEmoji(m.bellEnabled))
	return titleStyle.Width(m.width).Render(pad(text, m.width))
}

func renderRunsTable(m *Model) string {
	runs := m.tracker.VisibleRuns(m.showArchived)
	widths := calculateColumnWidths(m.width)

	builder := strings.Builder{}
	header := renderRow(tableHeaders(), widths, headerStyle)
	builder.WriteString(header)

	dataRows := m.dataRows()

	if len(runs) == 0 {
		linesUsed := 1
		for linesUsed < m.listArea.height {
			builder.WriteString("\n")
			builder.WriteString(strings.Repeat(" ", max(0, m.width)))
			linesUsed++
		}
		return builder.String()
	}

	start := m.scrollOffset
	end := min(start+dataRows, len(runs))
	linesUsed := 1

	for idx := start; idx < end; idx++ {
		builder.WriteString("\n")
		row := tableRowData(runs[idx])
		rowStr := renderRow(row, widths, rowStyle)
		if idx == m.selectedIndex && m.focus == focusRuns {
			rowStr = selectedRowStyle.Width(m.width).Render(rowStr)
		}
		builder.WriteString(rowStr)
		linesUsed++
	}

	for linesUsed < m.listArea.height {
		builder.WriteString("\n")
		builder.WriteString(strings.Repeat(" ", max(0, m.width)))
		linesUsed++
	}

	return builder.String()
}

func renderHelpText(m *Model) string {
	help := "[tab] focus • [o] open • [a] archive/restore • [A] view archived • [b] bell • [q] quit"
	return helpStyle.Width(m.width).Render(pad(help, m.width))
}

func renderStatusLine(m *Model) string {
	msg := m.status.text
	if msg == "" && m.pendingFetch {
		msg = "Fetching workflow runs…"
	}

	style := statusNeutralStyle
	switch m.status.kind {
	case statusError:
		style = statusErrorStyle
	case statusSuccess:
		style = statusSuccessStyle
	}

	if m.refreshing {
		refreshLabel := fmt.Sprintf("auto-refresh %s", m.spin.View())
		if msg == "" {
			msg = refreshLabel
		} else {
			msg = fmt.Sprintf("%s   %s", msg, refreshLabel)
		}
	}

	return style.Width(m.width).Render(pad(msg, m.width))
}

func renderInputField(m *Model) string {
	view := m.input.View()
	if m.focus == focusInput {
		return inputFocusedStyle.Render(view)
	}
	return inputStyle.Render(view)
}

func tableHeaders() []string {
	titles := make([]string, len(tableColumns))
	for i, c := range tableColumns {
		titles[i] = c.Title
	}
	return titles
}

func tableRowData(run *watch.TrackedRun) []string {
	owner, repo := splitRepo(run.Run.RepoFullName)
	data := []string{
		formatStatus(run.Run),
		repo,
		owner,
		run.Run.Target,
		run.Run.Name,
		run.Run.WorkflowName,
	}
	return data
}

func formatStatus(run githubclient.WorkflowRun) string {
	switch run.Status {
	case githubclient.RunStatusSuccess:
		return "✅"
	case githubclient.RunStatusFailed:
		return "❌"
	case githubclient.RunStatusPending:
		return "⏳"
	default:
		return "⏳"
	}
}

func renderRow(cells []string, widths []int, style lipgloss.Style) string {
	// Only include columns with non-zero widths
	var parts []string
	visibleCols := 0
	for i, cell := range cells {
		if widths[i] > 0 {
			cell = truncate(cell, widths[i])
			parts = append(parts, lipgloss.NewStyle().Width(widths[i]).Render(cell))
			visibleCols++
		}
	}
	row := strings.Join(parts, tableGap)
	rowWidth := lipgloss.Width(row)
	target := 0
	for _, w := range widths {
		if w > 0 {
			target += w
		}
	}
	if visibleCols > 0 {
		target += (visibleCols - 1) * lipgloss.Width(tableGap)
	}
	if rowWidth < target {
		row += strings.Repeat(" ", target-rowWidth)
	}
	return style.Render(row)
}

func calculateColumnWidths(total int) []int {
	if total <= 0 {
		total = 80
	}

	widths := make([]int, len(tableColumns))

	// Try to fit as many columns as possible, starting from the left
	// Drop columns from the right when space is insufficient
	for numCols := len(tableColumns); numCols >= 1; numCols-- {
		gaps := numCols - 1
		gapWidth := lipgloss.Width(tableGap)
		available := total - gaps*gapWidth
		if available < numCols {
			continue // Not even enough for 1 char per column
		}

		// Calculate minimum required and total weight for visible columns
		minRequired := 0
		totalWeight := 0.0
		for i := 0; i < numCols; i++ {
			minRequired += tableColumns[i].Min
			totalWeight += tableColumns[i].Weight
		}

		// If we can fit these columns with their minimums, calculate their widths
		if available >= minRequired {
			// Calculate widths using weighted distribution
			sum := 0
			for i := 0; i < numCols; i++ {
				col := tableColumns[i]
				// Normalize weight based on visible columns only
				normalizedWeight := col.Weight / totalWeight
				width := int(float64(available) * normalizedWeight)
				if width < col.Min {
					width = col.Min
				}
				widths[i] = width
				sum += width
			}

			// Adjust to match available width
			diff := available - sum
			if diff > 0 {
				widths[numCols-1] += diff
			}

			// Ensure no column is less than 1
			for i := 0; i < numCols; i++ {
				if widths[i] < 1 {
					widths[i] = 1
				}
			}

			// Set remaining columns to 0 (hidden)
			for i := numCols; i < len(tableColumns); i++ {
				widths[i] = 0
			}

			return widths
		}
	}

	// If we can't even fit one column with its minimum, just show first column
	widths[0] = max(1, total)
	for i := 1; i < len(widths); i++ {
		widths[i] = 0
	}
	return widths
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 1 {
		return lipgloss.NewStyle().MaxWidth(1).Render(text)
	}
	trimmed := lipgloss.NewStyle().MaxWidth(width - 1).Render(text)
	return trimmed + "…"
}

func pad(text string, width int) string {
	if width <= 0 {
		return text
	}
	return lipgloss.NewStyle().Width(width).Render(text)
}

func humanizeAgo(d time.Duration) string {
	if d < time.Second {
		return "just now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func titleCase(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(strings.ToLower(text))
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func bellEmoji(enabled bool) string {
	if enabled {
		return "🔔"
	}
	return "❌"
}
