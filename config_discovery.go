package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigNode represents a discovered config file and its location.
type ConfigNode struct {
	Path     string                 // Relative path to the config file
	Dir      string                 // Directory containing the config
	Config   *Config                // Parsed config content
	Children map[string]*ConfigNode // Child directories with configs
}

// ConfigTree represents the tree of discovered configs.
type ConfigTree struct {
	Root string      // Root directory that was walked
	Tree *ConfigNode // Root node of the config tree
}

// String returns a string representation of the ConfigNode.
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

// String returns a string representation of the ConfigTree.
func (ct *ConfigTree) String() string {
	if ct == nil {
		return "<nil ConfigTree>"
	}

	return fmt.Sprintf("ConfigTree(Root: %s)\n%s", ct.Root, ct.PrettyPrint())
}

// PrettyPrint returns a tree-like visualization of the config structure.
func (ct *ConfigTree) PrettyPrint() string {
	if ct == nil || ct.Tree == nil {
		return "<empty tree>"
	}

	var builder strings.Builder
	ct.printNode(&builder, ct.Tree, "", true)
	return builder.String()
}

// printNode recursively prints the tree structure with ASCII art.
func (ct *ConfigTree) printNode(builder *strings.Builder, node *ConfigNode, prefix string, isLast bool) {
	if node == nil {
		return
	}

	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	builder.WriteString(prefix + connector + nodeDescription(node) + "\n")

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

// nodeDescription renders a single node of the pretty-printed tree.
func nodeDescription(node *ConfigNode) string {
	if node.Config == nil {
		return fmt.Sprintf("📁 %s/", filepath.Base(node.Dir))
	}
	if filepath.Dir(node.Path) == "." && len(node.Children) == 0 {
		return fmt.Sprintf("📄 %s", filepath.Base(node.Path))
	}
	return fmt.Sprintf("📁 %s/ (📄 %s)", filepath.Base(node.Dir), filepath.Base(node.Path))
}

// DiscoverConfigs walks a directory tree and returns every config file found
// (see configFileNames). Inside a git work tree it uses git, so ignored files
// are skipped; elsewhere it walks the filesystem.
func DiscoverConfigs(rootDir string) (*ConfigTree, error) {
	if _, err := gitOutput(rootDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return discoverConfigsFileSystem(rootDir)
	}
	return discoverConfigsGit(rootDir)
}

// discoverConfigsGit uses git ls-files to find tracked and untracked (but not
// ignored) config files that exist in the work tree.
func discoverConfigsGit(rootDir string) (*ConfigTree, error) {
	args := make([]string, 0, 6+len(configFileNames))
	args = append(args, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--")
	for _, name := range configFileNames {
		args = append(args, ":(glob)**/"+name)
	}
	output, err := gitOutput(rootDir, args...)
	if err != nil {
		return nil, err
	}

	var present []string
	for _, p := range gitPaths(output) {
		// Deleted but not yet staged configs are still listed by --cached.
		if fileExists(filepath.Join(rootDir, p)) {
			present = append(present, p)
		}
	}
	return buildConfigTree(rootDir, selectConfigFiles(present))
}

// discoverConfigsFileSystem uses filepath.WalkDir to find config files.
func discoverConfigsFileSystem(rootDir string) (*ConfigTree, error) {
	var candidates []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if isConfigFileName(d.Name()) {
			relPath, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			candidates = append(candidates, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filesystem walk failed: %w", err)
	}

	return buildConfigTree(rootDir, selectConfigFiles(candidates))
}

func isConfigFileName(name string) bool {
	for _, n := range configFileNames {
		if name == n {
			return true
		}
	}
	return false
}

// selectConfigFiles keeps at most one config file per directory, preferring
// names that appear earlier in configFileNames.
func selectConfigFiles(paths []string) []string {
	priority := func(p string) int {
		base := filepath.Base(p)
		for i, n := range configFileNames {
			if base == n {
				return i
			}
		}
		return -1
	}

	best := make(map[string]string)
	for _, p := range paths {
		p = filepath.Clean(p)
		if priority(p) < 0 {
			continue
		}
		dir := filepath.Dir(p)
		if cur, ok := best[dir]; !ok || priority(p) < priority(cur) {
			best[dir] = p
		}
	}

	selected := make([]string, 0, len(best))
	for _, p := range best {
		selected = append(selected, p)
	}
	sort.Strings(selected)
	return selected
}

// buildConfigTree creates a ConfigTree from a list of config file paths.
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
			return nil, fmt.Errorf("failed to add config %s: %w", configPath, err)
		}
	}

	return tree, nil
}

// addConfigToTree adds a single config file to the tree structure.
func addConfigToTree(tree *ConfigTree, configPath string) error {
	// Parse the config file
	config, err := parseConfig(filepath.Join(tree.Root, configPath))
	if err != nil {
		return err
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

// parseConfig loads and validates a config file.
func parseConfig(configPath string) (*Config, error) {
	return loadConfig(configPath)
}

// FlattenConfigs returns all config nodes from the tree as a flat list.
func (ct *ConfigTree) FlattenConfigs() []*ConfigNode {
	if ct == nil || ct.Tree == nil {
		return nil
	}

	var configs []*ConfigNode
	ct.flattenNode(ct.Tree, &configs)
	return configs
}

// flattenNode recursively traverses the tree and collects all config nodes.
func (ct *ConfigTree) flattenNode(node *ConfigNode, configs *[]*ConfigNode) {
	if node == nil {
		return
	}

	// Add this node if it has a config
	if node.Config != nil {
		*configs = append(*configs, node)
	}

	// Recursively process children
	for _, child := range node.Children {
		ct.flattenNode(child, configs)
	}
}
