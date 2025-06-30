package tui

import (
	"fmt"
	"strings"
	"time"
)

// renderStats renders the statistics section
func (r *Renderer) renderStats(vm *ViewModel, width int) string {
	var b strings.Builder

	// Header
	b.WriteString(HeaderStyle.Render("Usage Statistics") + "\n\n")

	// Calculate available width for stats table (account for box padding)
	availableWidth := width - 6 // Leave margin for box borders and padding
	if availableWidth < 60 {
		// Render compact stats for narrow terminals
		return r.renderCompactStats(vm)
	}

	// Create table headers
	headers := []string{"Model Tier", "Reqs", "Limited", "Cache", "Total", "Cost ($)"}

	// Calculate dynamic column widths based on available space
	colWidths := CalculateStatsColumnWidths(availableWidth)

	// Render header row
	for i, header := range headers {
		cell := TableHeaderStyle.Render(PadRight(header, colWidths[i]))
		b.WriteString(cell)
	}
	b.WriteString("\n")

	// Separator line
	for _, width := range colWidths {
		b.WriteString(strings.Repeat("─", width))
	}
	b.WriteString("\n")

	stats := vm.Stats()

	// Base (Haiku) row
	baseRow := []string{
		BaseStyle.Bold(true).Render("Base (Haiku)"),
		fmt.Sprintf("%d", stats.BaseRequests()),
		FormatTokenCount(stats.BaseTokens().Limited()),
		FormatTokenCount(stats.BaseTokens().Cache()),
		FormatTokenCount(stats.BaseTokens().Total()),
		fmt.Sprintf("%.6f", stats.BaseCost().Amount()),
	}
	for i, cell := range baseRow {
		if i == 0 {
			b.WriteString(PadRight(cell, colWidths[i]))
		} else {
			b.WriteString(BaseStyle.Render(PadRight(cell, colWidths[i])))
		}
	}
	b.WriteString("\n")

	// Premium (S/O) row
	premiumRow := []string{
		PremiumStyle.Bold(true).Render("Premium (S/O)"),
		fmt.Sprintf("%d", stats.PremiumRequests()),
		FormatTokenCount(stats.PremiumTokens().Limited()),
		FormatTokenCount(stats.PremiumTokens().Cache()),
		FormatTokenCount(stats.PremiumTokens().Total()),
		fmt.Sprintf("%.6f", stats.PremiumCost().Amount()),
	}
	for i, cell := range premiumRow {
		if i == 0 {
			b.WriteString(PadRight(cell, colWidths[i]))
		} else {
			b.WriteString(PremiumStyle.Render(PadRight(cell, colWidths[i])))
		}
	}
	b.WriteString("\n")

	// Separator before total
	for _, width := range colWidths {
		b.WriteString(strings.Repeat("─", width))
	}
	b.WriteString("\n")

	// Total row
	totalRow := []string{
		StatStyle.Bold(true).Render("Total"),
		fmt.Sprintf("%d", stats.TotalRequests()),
		FormatTokenCount(stats.TotalTokens().Limited()),
		FormatTokenCount(stats.TotalTokens().Cache()),
		FormatTokenCount(stats.TotalTokens().Total()),
		fmt.Sprintf("%.6f", stats.TotalCost().Amount()),
	}
	for i, cell := range totalRow {
		if i == 0 {
			b.WriteString(PadRight(cell, colWidths[i]))
		} else {
			b.WriteString(StatStyle.Render(PadRight(cell, colWidths[i])))
		}
	}

	// Add progress bar section if block is configured with limit
	if vm.Block() != nil && vm.Block().HasLimit() {
		b.WriteString("\n\n")
		b.WriteString(r.renderBlockProgress(vm))
	} else if vm.Block() == nil {
		// Show help message if no block is configured
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Use -b 5am to track token limits"))
	}

	return b.String()
}

// renderCompactStats renders a compact version of stats for narrow terminals
func (r *Renderer) renderCompactStats(vm *ViewModel) string {
	var b strings.Builder

	// Header
	b.WriteString(HeaderStyle.Render("Usage Statistics") + "\n\n")

	stats := vm.Stats()

	// Compact format for narrow terminals
	b.WriteString(StatStyle.Render("Total Requests: "))
	b.WriteString(fmt.Sprintf("%d\n", stats.TotalRequests()))

	b.WriteString(StatStyle.Render("Total Tokens: "))
	b.WriteString(fmt.Sprintf("%s\n", FormatTokenCount(stats.TotalTokens().Total())))

	b.WriteString(StatStyle.Render("Total Cost: "))
	b.WriteString(fmt.Sprintf("$%.6f\n", stats.TotalCost().Amount()))

	b.WriteString("\n")
	b.WriteString(BaseStyle.Render("Base: "))
	b.WriteString(fmt.Sprintf("%d reqs, %s tokens, $%.6f\n",
		stats.BaseRequests(),
		FormatTokenCount(stats.BaseTokens().Total()),
		stats.BaseCost().Amount()))

	b.WriteString(PremiumStyle.Render("Premium: "))
	b.WriteString(fmt.Sprintf("%d reqs, %s tokens, $%.6f",
		stats.PremiumRequests(),
		FormatTokenCount(stats.PremiumTokens().Total()),
		stats.PremiumCost().Amount()))

	// Add progress bar section if block is configured with limit
	if vm.Block() != nil && vm.Block().HasLimit() {
		b.WriteString("\n\n")
		b.WriteString(r.renderBlockProgress(vm))
	} else if vm.Block() == nil {
		// Show help message if no block is configured
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Use -b 5am to track token limits"))
	}

	return b.String()
}

// renderBlockProgress renders the block progress bar section
func (r *Renderer) renderBlockProgress(vm *ViewModel) string {
	var b strings.Builder

	blockStats := vm.BlockStats()

	// Calculate progress using Block entity method
	percentage := vm.Block().CalculateProgress(blockStats.PremiumTokens())

	if percentage > 100 {
		percentage = 100
	}

	// Calculate time remaining until next block
	now := time.Now().UTC()
	var timeRemaining time.Duration
	if now.Before(vm.Block().EndAt()) {
		timeRemaining = vm.Block().EndAt().Sub(now)
	}

	// Block header
	blockTime := ""
	if vm.Block() != nil {
		blockTime = vm.Block().FormatBlockTime(vm.Timezone())
	}
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("Block Progress (%s)", blockTime)))
	b.WriteString("\n\n")

	// Progress bar
	progressBar := RenderProgressBar(percentage, 40)
	b.WriteString(progressBar)
	b.WriteString(" ")
	used := blockStats.PremiumTokens().Limited()
	limit := int64(vm.Block().TokenLimit())
	b.WriteString(StatStyle.Render(fmt.Sprintf("%.1f%% (%s/%s tokens)", percentage, FormatTokenCount(used), FormatTokenCount(limit))))
	b.WriteString("\n")

	// Time remaining
	if timeRemaining > 0 {
		b.WriteString(HelpStyle.Render(fmt.Sprintf("Time remaining: %s", FormatDurationFromTime(timeRemaining))))
	} else {
		b.WriteString(HelpStyle.Render("Block expired"))
	}

	return b.String()
}
