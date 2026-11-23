package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	treeCommitConfigFile = ".tree-commit-config.yaml"
	preCommitConfigFile  = ".pre-commit-config.yaml"
	manifestFile         = ".pre-commit-hooks.yaml"

	repoLocal = "local"
	repoMeta  = "meta"

	langSystem = "system"
	langScript = "script"
	langPython = "python"
	langFail   = "fail"
	langPygrep = "pygrep"

	windows = "windows"
)

// configFileNames lists the config file names looked up in every directory, in
// priority order. Only the first match in a directory is used.
var configFileNames = []string{treeCommitConfigFile, preCommitConfigFile}

// Config is the content of a single .tree-commit-config.yaml (or .pre-commit-config.yaml).
type Config struct {
	Repos         []RepoConfig `yaml:"repos"`
	Files         string       `yaml:"files"`
	Exclude       string       `yaml:"exclude"`
	DefaultStages []string     `yaml:"default_stages"`
}

// RepoConfig is one entry of the top-level repos list.
// Hooks are kept raw so that remote hooks can be merged on top of the
// repository's .pre-commit-hooks.yaml manifest, field by field.
type RepoConfig struct {
	Repo  string           `yaml:"repo"`
	Rev   string           `yaml:"rev"`
	Hooks []map[string]any `yaml:"hooks"`
}

// Hook is a fully resolved hook definition. The fields mirror pre-commit's hook
// schema, plus the tree-commit specific include_folders, exclude_folders and disable.
type Hook struct {
	ID                     string   `yaml:"id"`
	Name                   string   `yaml:"name"`
	Entry                  string   `yaml:"entry"`
	Language               string   `yaml:"language"`
	Files                  string   `yaml:"files"`
	Exclude                string   `yaml:"exclude"`
	Types                  []string `yaml:"types"`
	TypesOr                []string `yaml:"types_or"`
	ExcludeTypes           []string `yaml:"exclude_types"`
	Args                   []string `yaml:"args"`
	PassFilenames          *bool    `yaml:"pass_filenames"`
	AlwaysRun              bool     `yaml:"always_run"`
	Stages                 []string `yaml:"stages"`
	AdditionalDependencies []string `yaml:"additional_dependencies"`
	RequireSerial          bool     `yaml:"require_serial"`
	Verbose                bool     `yaml:"verbose"`
	IncludeFolders         []string `yaml:"include_folders"`
	ExcludeFolders         []string `yaml:"exclude_folders"`
	Disable                bool     `yaml:"disable"`
}

func (h *Hook) passFilenames() bool {
	return h.PassFilenames == nil || *h.PassFilenames
}

// legacyStages maps pre-commit's old stage names to git hook names.
var legacyStages = map[string]string{
	"commit":       "pre-commit",
	"merge-commit": "pre-merge-commit",
	"push":         "pre-push",
}

func normalizeStage(stage string) string {
	if s, ok := legacyStages[stage]; ok {
		return s
	}
	return stage
}

// runsInStage reports whether the hook is configured to run for the given git
// hook stage. Hooks without stages fall back to the config's default_stages.
func (h *Hook) runsInStage(stage string, defaults []string) bool {
	stages := h.Stages
	if len(stages) == 0 {
		stages = defaults
	}
	if len(stages) == 0 {
		return true
	}
	stage = normalizeStage(stage)
	for _, s := range stages {
		if normalizeStage(s) == stage {
			return true
		}
	}
	return false
}

// loadConfig reads and validates a config file.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from config discovery
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config file %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	for i, repo := range c.Repos {
		if repo.Repo == "" {
			return fmt.Errorf("repos[%d]: missing required field 'repo'", i)
		}
		if repo.Repo != repoLocal && repo.Repo != repoMeta && repo.Rev == "" {
			return fmt.Errorf("repos[%d] (%s): missing required field 'rev'", i, repo.Repo)
		}
		// Both values are passed to git; a leading dash would be parsed as an option
		// (e.g. --upload-pack=<command>).
		if strings.HasPrefix(repo.Repo, "-") || strings.HasPrefix(repo.Rev, "-") {
			return fmt.Errorf("repos[%d]: 'repo' and 'rev' must not start with '-'", i)
		}
		for j, raw := range repo.Hooks {
			if id, _ := raw["id"].(string); id == "" {
				return fmt.Errorf("repos[%d].hooks[%d]: missing required field 'id'", i, j)
			}
		}
	}
	return nil
}

// decodeHook converts a raw hook map into a Hook.
func decodeHook(raw map[string]any) (*Hook, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to encode hook: %w", err)
	}
	hook := &Hook{}
	if err := yaml.Unmarshal(data, hook); err != nil {
		return nil, fmt.Errorf("failed to decode hook: %w", err)
	}
	return hook, nil
}

// decodeLocalHook decodes a hook from a `repo: local` block and applies defaults.
func decodeLocalHook(raw map[string]any) (*Hook, error) {
	hook, err := decodeHook(raw)
	if err != nil {
		return nil, err
	}
	if hook.Name == "" {
		hook.Name = hook.ID
	}
	if hook.Language == "" {
		hook.Language = langSystem
	}
	if hook.Entry == "" && !hook.Disable {
		return nil, errors.New("local hook is missing required field 'entry'")
	}
	return hook, nil
}

// mergeHook overlays the user's hook configuration on top of the manifest definition.
func mergeHook(manifest, override map[string]any) map[string]any {
	merged := make(map[string]any, len(manifest)+len(override))
	for k, v := range manifest {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// loadManifest reads a .pre-commit-hooks.yaml file and indexes its hooks by id.
func loadManifest(path string) (map[string]map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is inside the hook repository checkout
	if err != nil {
		return nil, fmt.Errorf("failed to read hook manifest: %w", err)
	}
	var hooks []map[string]any
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		return nil, fmt.Errorf("failed to parse hook manifest %s: %w", path, err)
	}
	byID := make(map[string]map[string]any, len(hooks))
	for _, h := range hooks {
		if id, _ := h["id"].(string); id != "" {
			byID[id] = h
		}
	}
	return byID, nil
}
