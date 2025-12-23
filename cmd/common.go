package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ptone/scion/pkg/config"
	"github.com/ptone/scion/pkg/util"
)

func DeleteAgentFiles(agentName string) error {
	var agentsDirs []string
	if projectDir, err := config.GetResolvedProjectDir(grovePath); err == nil {
		agentsDirs = append(agentsDirs, filepath.Join(projectDir, "agents"))
	}
	// Also check global just in case
	if globalDir, err := config.GetGlobalAgentsDir(); err == nil {
		agentsDirs = append(agentsDirs, globalDir)
	}

	for _, dir := range agentsDirs {
		agentDir := filepath.Join(dir, agentName)
		if _, err := os.Stat(agentDir); err != nil {
			continue
		}

		agentWorkspace := filepath.Join(agentDir, "workspace")
		// Check if it's a worktree before trying to remove it
		if _, err := os.Stat(filepath.Join(agentWorkspace, ".git")); err == nil {
			fmt.Printf("Removing git worktree for agent '%s'...\n", agentName)
			if err := util.RemoveWorktree(agentWorkspace); err != nil {
				fmt.Printf("Warning: failed to remove worktree at %s: %v\n", agentWorkspace, err)
			}
		}

		// Also ensure the agent directory is cleaned up
		fmt.Printf("Removing agent directory for '%s'...\n", agentName)
		if err := os.RemoveAll(agentDir); err != nil {
			return fmt.Errorf("failed to remove agent directory: %w", err)
		}
	}
	return nil
}