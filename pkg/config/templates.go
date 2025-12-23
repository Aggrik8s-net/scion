package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Template struct {
	Name string
	Path string
}

type AgentConfig struct {
	Grove  string `json:"grove"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

type ScionConfig struct {
	UnixUsername string       `json:"unix_username"`
	Image        string       `json:"image"`
	Detached     *bool        `json:"detached"`
	UseTmux      bool         `json:"use_tmux"`
	Model        string       `json:"model"`
	Agent        *AgentConfig `json:"agent,omitempty"`
}

func (c *ScionConfig) IsDetached() bool {
	if c.Detached == nil {
		return true
	}
	return *c.Detached
}

func (t *Template) LoadConfig() (*ScionConfig, error) {
	path := filepath.Join(t.Path, "scion.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ScionConfig{}, nil
		}
		return nil, err
	}

	var cfg ScionConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func FindTemplate(name string) (*Template, error) {
	// 1. Check project-local templates
	projectTemplatesDir, err := GetProjectTemplatesDir()
	if err == nil {
		path := filepath.Join(projectTemplatesDir, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return &Template{Name: name, Path: path}, nil
		}
	}

	// 2. Check global templates
	globalTemplatesDir, err := GetGlobalTemplatesDir()
	if err == nil {
		path := filepath.Join(globalTemplatesDir, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return &Template{Name: name, Path: path}, nil
		}
	}

	return nil, fmt.Errorf("template %s not found", name)
}

// GetTemplateChain returns a list of templates in inheritance order (base first)
func GetTemplateChain(name string) ([]*Template, error) {
	var chain []*Template

	// Always start with default if it's not the requested template
	if name != "default" {
		def, err := FindTemplate("default")
		if err == nil {
			chain = append(chain, def)
		}
	}

	tpl, err := FindTemplate(name)
	if err != nil {
		return nil, err
	}
	chain = append(chain, tpl)

	return chain, nil
}

func ListTemplates() ([]*Template, error) {
	templates := make(map[string]*Template)

	// Helper to scan a directory for templates
	scan := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				templates[e.Name()] = &Template{
					Name: e.Name(),
					Path: filepath.Join(dir, e.Name()),
				}
			}
		}
	}

	// 1. Scan global templates (lower precedence in map)
	if globalDir, err := GetGlobalTemplatesDir(); err == nil {
		scan(globalDir)
	}

	// 2. Scan project templates (higher precedence)
	if projectDir, err := GetProjectTemplatesDir(); err == nil {
		scan(projectDir)
	}

	var list []*Template
	for _, t := range templates {
		list = append(list, t)
	}
	return list, nil
}

