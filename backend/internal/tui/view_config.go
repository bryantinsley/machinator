package tui

import (
	"fmt"

	"github.com/bryantinsley/machinator/backend/internal/config"
)

// buildConfigView creates the config display for the right pane.
func (t *TUI) buildConfigView() string {
	var content string

	// Global config
	content += "[yellow]Global Configuration[-]\n"
	content += "─────────────────────\n"
	content += fmt.Sprintf("default_agent_count: [white]%d[-]\n", t.cfg.DefaultAgentCount)
	content += fmt.Sprintf("hide_commit_authors: [white]%v[-]\n", t.cfg.HideCommitAuthors)
	content += "\n"

	content += "[yellow]Agent Timeouts[-]\n"
	content += fmt.Sprintf("  idle: [white]%s[-]\n", t.cfg.Timeouts.Idle.Duration())
	content += fmt.Sprintf("  max_runtime: [white]%s[-]\n", t.cfg.Timeouts.MaxRuntime.Duration())
	content += "\n"

	content += "[yellow]Intervals[-]\n"
	content += fmt.Sprintf("  assigner: [white]%s[-]\n", t.cfg.Intervals.Assigner.Duration())
	content += fmt.Sprintf("  quota_refresh: [white]%s[-]\n", t.cfg.Intervals.QuotaRefresh.Duration())
	content += fmt.Sprintf("  agent_watch: [white]%s[-]\n", t.cfg.Intervals.AgentWatch.Duration())

	// Project config
	content += "\n[yellow]Project Configuration[-]\n"
	content += "──────────────────────\n"
	if t.projCfg != nil {
		content += fmt.Sprintf("repo: [white]%s[-]\n", t.projCfg.Repo)
		content += fmt.Sprintf("branch: [white]%s[-]\n", t.projCfg.Branch)
		content += fmt.Sprintf("simple_model: [white]%s[-]\n", t.projCfg.SimpleModelName)
		content += fmt.Sprintf("complex_model: [white]%s[-]\n", t.projCfg.ComplexModelName)
	} else {
		content += "[gray]No project loaded[-]\n"
	}

	// Paths
	content += "\n[yellow]Config Paths[-]\n"
	content += fmt.Sprintf("global:  [gray]%s[-]\n", config.ConfigPath())
	content += fmt.Sprintf("project: [gray]%s[-]\n", t.projectConfigPath)

	return content
}
