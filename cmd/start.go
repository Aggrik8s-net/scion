package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ptone/scion/pkg/config"
	"github.com/ptone/scion/pkg/runtime"
	"github.com/ptone/scion/pkg/util"
	"github.com/spf13/cobra"
)

var (
	agentName    string
	templateName string
	agentImage   string
	noAuth       bool
	attach       bool
	model        string
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start <agent-name> <task...>",
	Short: "Launch a new scion agent",
	Long: `Provision and launch a new isolated LLM agent to perform a specific task.
The agent will be created from a template and run in a detached container.

The agent-name is required as the first argument. All subsequent arguments 
form the task prompt.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName = args[0]
		task := strings.Join(args[1:], " ")

		fmt.Printf("Starting agent '%s' for task: %s\n", agentName, task)

		// 1. Prepare agent directories
		var agentsDir string
		projectDir, err := config.GetResolvedProjectDir(grovePath)
		if err != nil {
			return err
		}

		groveName := config.GetGroveName(projectDir)

		// Verify .gitignore if in a repo
		if util.IsGitRepo() {
			// Find the projectDir relative to repo root if possible
			root, err := util.RepoRoot()
			if err == nil {
				rel, err := filepath.Rel(root, projectDir)
				if err == nil && !strings.HasPrefix(rel, "..") {
					agentsPath := filepath.Join(rel, "agents")
					if !util.IsIgnored(agentsPath + "/") {
						return fmt.Errorf("security error: '%s/' must be in .gitignore when using a project-local grove", agentsPath)
					}
				}
			}
		}
		agentsDir = filepath.Join(projectDir, "agents")

		agentDir := filepath.Join(agentsDir, agentName)
		agentHome := filepath.Join(agentDir, "home")
		agentWorkspace := filepath.Join(agentDir, "workspace")

		if err := os.MkdirAll(agentHome, 0755); err != nil {
			return fmt.Errorf("failed to create agent home: %w", err)
		}

		if util.IsGitRepo() {
			fmt.Printf("Creating git worktree for agent '%s'...\n", agentName)
			// Remove existing workspace dir if it exists to allow worktree add
			os.RemoveAll(agentWorkspace)
			if err := util.CreateWorktree(agentWorkspace, agentName); err != nil {
				return fmt.Errorf("failed to create git worktree: %w", err)
			}
		} else {
			if err := os.MkdirAll(agentWorkspace, 0755); err != nil {
				return fmt.Errorf("failed to create agent workspace: %w", err)
			}
		}

		// 2. Load and copy templates
		chain, err := config.GetTemplateChain(templateName)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Track image from templates
		resolvedImage := ""
		var finalScionCfg *config.ScionConfig

		for _, tpl := range chain {
			fmt.Printf("Applying template: %s\n", tpl.Name)
			if err := util.CopyDir(tpl.Path, agentHome); err != nil {
				return fmt.Errorf("failed to copy template %s: %w", tpl.Name, err)
			}

			// Load scion.json from this template to see if it specifies an image
			tplCfg, err := tpl.LoadConfig()
			if err == nil {
				finalScionCfg = tplCfg
				if tplCfg.Image != "" {
					resolvedImage = tplCfg.Image
				}
			}
		}

		// Update agent-specific scion.json
		if finalScionCfg == nil {
			finalScionCfg = &config.ScionConfig{}
		}
		finalScionCfg.Agent = &config.AgentConfig{
			Grove: groveName,
			Name:  agentName,
		}
		agentCfgData, _ := json.MarshalIndent(finalScionCfg, "", "  ")
		os.WriteFile(filepath.Join(agentHome, "scion.json"), agentCfgData, 0644)

		// Flag takes ultimate precedence
		if agentImage != "" {
			resolvedImage = agentImage
		}
		if resolvedImage == "" {
			resolvedImage = "gemini-cli-sandbox"
		}

		// 3. Propagate credentials
		var auth config.AuthConfig
		if !noAuth {
			// Load agent settings from the newly prepared home directory
			agentSettingsPath := filepath.Join(agentHome, ".gemini", "settings.json")
			agentSettings, _ := config.LoadGeminiSettings(agentSettingsPath)
			auth = config.DiscoverAuth(agentSettings)
		}

		// 4. Launch container
		rt := runtime.GetRuntime()

		// Determine detached mode, tmux, and username from templates (last one wins)
		detached := true
		useTmux := false
		resolvedModel := "flash"
		unixUsername := "node"
		for _, tpl := range chain {
			tplCfg, err := tpl.LoadConfig()
			if err == nil {
				detached = tplCfg.IsDetached()
				if tplCfg.UseTmux {
					useTmux = true
				}
				if tplCfg.Model != "" {
					resolvedModel = tplCfg.Model
				}
				if tplCfg.UnixUsername != "" {
					unixUsername = tplCfg.UnixUsername
				}
			}
		}

		// -a flag overrides detached config
		if cmd.Flags().Changed("attach") {
			detached = !attach
		}

		if model != "" {
			resolvedModel = model
		}

		if useTmux {
			tmuxImage := resolvedImage
			if !strings.Contains(tmuxImage, ":") {
				tmuxImage = tmuxImage + ":tmux"
			} else {
				parts := strings.SplitN(resolvedImage, ":", 2)
				tmuxImage = parts[0] + ":tmux"
			}

			exists, err := rt.ImageExists(context.Background(), tmuxImage)
			if err != nil || !exists {
				return fmt.Errorf("tmux support requested but image '%s' not found. Please ensure the image has a :tmux tag.", tmuxImage)
			}
			resolvedImage = tmuxImage
		}

		agentEnv := []string{
			fmt.Sprintf("GEMINI_AGENT_NAME=%s", agentName),
		}
		if !strings.HasPrefix(strings.TrimSpace(config.DefaultSystemPrompt), "# Placeholder") {
			agentEnv = append(agentEnv, fmt.Sprintf("GEMINI_SYSTEM_MD=/home/%s/.gemini/system_prompt.md", unixUsername))
		}

		runCfg := runtime.RunConfig{
			Name:         agentName,
			UnixUsername: unixUsername,
			Image:        resolvedImage,
			HomeDir:      agentHome,
			Workspace:    agentWorkspace,
			Auth:         auth,
			UseTmux:      useTmux,
			Model:        resolvedModel,
			Task:         task,
			Env:          agentEnv,
			Labels: map[string]string{
				"scion.agent":      "true",
				"scion.name":       agentName,
				"scion.grove":      groveName,
				"scion.grove_path": projectDir,
			},
		}

		id, err := rt.Run(context.Background(), runCfg)
		if err != nil {
			return fmt.Errorf("failed to launch container: %w", err)
		}

		if detached {
			fmt.Printf("Agent '%s' launched successfully (ID: %s)\n", agentName, id)
		} else {
			fmt.Printf("Attaching to agent '%s'...\n", agentName)
			return rt.Attach(context.Background(), id)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&templateName, "type", "t", "default", "Template to use")
	startCmd.Flags().StringVarP(&agentImage, "image", "i", "", "Container image to use (overrides template)")
	startCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Disable authentication propagation")
	startCmd.Flags().BoolVarP(&attach, "attach", "a", false, "Attach to the agent TTY after starting")
	startCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (overrides template)")
}
			
