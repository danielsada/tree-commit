package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// ConfigNode represents a discovered config file and its location
type ConfigNode struct {
	Path     string                 // Relative path to the config file
	Dir      string                 // Directory containing the config
	Config   *viper.Viper          // Parsed config content
	Children map[string]*ConfigNode // Child directories with configs
}

// ConfigTree represents the tree of discovered configs
type ConfigTree struct {
	Root string      // Root directory that was walked
	Tree *ConfigNode // Root node of the config tree
}

// String returns a string representation of the ConfigNode
func (cn *ConfigNode) String() string {
	if cn == nil {
		return "<nil>"
	}

	var parts []string
	if cn.Config != nil {
		parts = append(parts, fmt.Sprintf("Config: %s", cn.Path))
	} else {
		parts = append(parts, fmt.Sprintf("Dir: %s", cn.Dir))
	}

	if len(cn.Children) > 0 {
		var childNames []string
		for name := range cn.Children {
			childNames = append(childNames, name)
		}
		sort.Strings(childNames)
		parts = append(parts, fmt.Sprintf("Children: [%s]", strings.Join(childNames, ", ")))
	}

	return strings.Join(parts, ", ")
}

// String returns a string representation of the ConfigTree
func (ct *ConfigTree) String() string {
	if ct == nil {
		return "<nil ConfigTree>"
	}

	return fmt.Sprintf("ConfigTree(Root: %s)\n%s", ct.Root, ct.PrettyPrint())
}

// PrettyPrint returns a tree-like visualization of the config structure
func (ct *ConfigTree) PrettyPrint() string {
	if ct == nil || ct.Tree == nil {
		return "<empty tree>"
	}

	var builder strings.Builder
	ct.printNode(&builder, ct.Tree, "", true)
	return builder.String()
}

// printNode recursively prints the tree structure with ASCII art
func (ct *ConfigTree) printNode(builder *strings.Builder, node *ConfigNode, prefix string, isLast bool) {
	if node == nil {
		return
	}

	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	var nodeDesc string
	if node.Config != nil && len(node.Children) == 0 {
		// Leaf node with config - show directory name
		dirName := filepath.Base(filepath.Dir(node.Path))
		if dirName == "." {
			// Root level config
			nodeDesc = fmt.Sprintf("📄 %s", filepath.Base(node.Path))
		} else {
			// Directory level config
			nodeDesc = fmt.Sprintf("📁 %s/ (📄 %s)", dirName, filepath.Base(node.Path))
		}
	} else if node.Config != nil && len(node.Children) > 0 {
		// Directory with both config and children - show as directory
		dirName := filepath.Base(filepath.Dir(node.Path))
		nodeDesc = fmt.Sprintf("📁 %s/ (📄 %s)", dirName, filepath.Base(node.Path))
	} else {
		// Directory node without config
		dirName := filepath.Base(node.Dir)
		if dirName == "." || dirName == "" {
			dirName = filepath.Base(node.Dir)
		}
		nodeDesc = fmt.Sprintf("📁 %s/", dirName)
	}

	builder.WriteString(prefix + connector + nodeDesc + "\n")

	// Prepare prefix for children
	var childPrefix string
	if isLast {
		childPrefix = prefix + "    "
	} else {
		childPrefix = prefix + "│   "
	}

	// Print children
	if len(node.Children) > 0 {
		var childNames []string
		for name := range node.Children {
			childNames = append(childNames, name)
		}
		sort.Strings(childNames)

		for i, childName := range childNames {
			child := node.Children[childName]
			isChildLast := i == len(childNames)-1
			ct.printNode(builder, child, childPrefix, isChildLast)
		}
	}
}

// DiscoverConfigs walks a directory tree and returns all .tree-commit-config.yaml files
// It attempts to use git first, then falls back to filesystem walking
func DiscoverConfigs(rootDir string) (*ConfigTree, error) {
	// Try git mode first
	if configTree, err := discoverConfigsGit(rootDir); err == nil {
		return configTree, nil
	}

	// Fall back to filesystem walking
	return discoverConfigsFileSystem(rootDir)
}

// discoverConfigsGit uses git ls-files to find config files
func discoverConfigsGit(rootDir string) (*ConfigTree, error) {
	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = rootDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not in a git repository: %v", err)
	}

	// Use git ls-files to find all .tree-commit-config.yaml files (including untracked)
	cmd = exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = rootDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %v", err)
	}

	allFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(allFiles) == 1 && allFiles[0] == "" {
		allFiles = []string{}
	}

	// Filter for .tree-commit-config.yaml files
	var configPaths []string
	for _, file := range allFiles {
		if strings.HasSuffix(file, ".tree-commit-config.yaml") {
			configPaths = append(configPaths, file)
		}
	}

	return buildConfigTree(rootDir, configPaths)
}

// discoverConfigsFileSystem uses os.WalkDir to find config files
func discoverConfigsFileSystem(rootDir string) (*ConfigTree, error) {
	var configPaths []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Name() == ".tree-commit-config.yaml" {
			// Convert to relative path
			relPath, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			configPaths = append(configPaths, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("filesystem walk failed: %v", err)
	}

	return buildConfigTree(rootDir, configPaths)
}

// buildConfigTree creates a ConfigTree from a list of config file paths
func buildConfigTree(rootDir string, configPaths []string) (*ConfigTree, error) {
	tree := &ConfigTree{
		Root: rootDir,
		Tree: &ConfigNode{
			Dir:      rootDir,
			Children: make(map[string]*ConfigNode),
		},
	}

	for _, configPath := range configPaths {
		if err := addConfigToTree(tree, configPath); err != nil {
			return nil, fmt.Errorf("failed to add config %s: %v", configPath, err)
		}
	}

	return tree, nil
}

// addConfigToTree adds a single config file to the tree structure
func addConfigToTree(tree *ConfigTree, configPath string) error {
	// Parse the config file
	config, err := parseConfig(filepath.Join(tree.Root, configPath))
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	// Split path into directory components
	dir := filepath.Dir(configPath)
	var pathComponents []string
	if dir != "." {
		pathComponents = strings.Split(dir, string(filepath.Separator))
	}

	// Navigate/create the tree structure
	currentNode := tree.Tree
	currentPath := ""

	for _, component := range pathComponents {
		if currentPath == "" {
			currentPath = component
		} else {
			currentPath = filepath.Join(currentPath, component)
		}

		if currentNode.Children[component] == nil {
			currentNode.Children[component] = &ConfigNode{
				Dir:      currentPath,
				Children: make(map[string]*ConfigNode),
			}
		}
		currentNode = currentNode.Children[component]
	}

	// Set the config on the final node
	currentNode.Config = config
	currentNode.Path = configPath

	return nil
}

// parseConfig loads and parses a .tree-commit-config.yaml file
func parseConfig(configPath string) (*viper.Viper, error) {
	config := viper.New()
	config.SetConfigFile(configPath)
	config.SetConfigType("yaml")

	if err := config.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configPath, err)
	}

	return config, nil
}