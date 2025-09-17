package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverConfigsInDirectory(t *testing.T) {
	// Test walking example_repos/example_monorepo and finding all configs
	configTree, err := DiscoverConfigs("example_repos/example_monorepo")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Debug: Print the discovered tree
	t.Logf("Discovered tree:\n%s", configTree.String())

	// Should find configs in a tree structure
	// Root level should have 5 main directories
	expectedMainDirs := []string{"python_lib", "go_lib", "frontend", "docs", "scripts"}

	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node")
	}

	t.Logf("Root tree has %d children", len(configTree.Tree.Children))
	for name := range configTree.Tree.Children {
		t.Logf("Found child: %s", name)
	}

	for _, dir := range expectedMainDirs {
		if configTree.Tree.Children[dir] == nil {
			t.Errorf("Expected to find config in directory: %s", dir)
		}
	}

	// Check python_lib has nested structure
	pythonLibNode := configTree.Tree.Children["python_lib"]
	if pythonLibNode == nil {
		t.Fatal("Expected python_lib config node")
	}

	t.Logf("python_lib node has %d children", len(pythonLibNode.Children))
	for name := range pythonLibNode.Children {
		t.Logf("python_lib child: %s", name)
	}

	// python_lib should have src as child
	if pythonLibNode.Children["src"] == nil {
		t.Error("Expected python_lib/src config node")
		return // Don't continue if this fails
	}

	// python_lib/src should have core and utils as children
	srcNode := pythonLibNode.Children["src"]
	if srcNode == nil {
		t.Error("srcNode is nil")
		return
	}

	t.Logf("src node has %d children", len(srcNode.Children))
	for name := range srcNode.Children {
		t.Logf("src child: %s", name)
	}

	if srcNode.Children["core"] == nil {
		t.Error("Expected python_lib/src/core config node")
	}
	if srcNode.Children["utils"] == nil {
		t.Error("Expected python_lib/src/utils config node")
		return
	}

	// python_lib/src/utils should have helpers as child
	utilsNode := srcNode.Children["utils"]
	if utilsNode == nil {
		t.Error("utilsNode is nil")
		return
	}

	if utilsNode.Children["helpers"] == nil {
		t.Error("Expected python_lib/src/utils/helpers config node")
	}
}

func TestDiscoverConfigsInSingleLibrary(t *testing.T) {
	// Test walking example_repos/example_monorepo/python_lib (library with nested configs)
	configTree, err := DiscoverConfigs("example_repos/example_monorepo/python_lib")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Should find 1 config at root level
	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node")
	}

	// Verify the config is at the root and has content
	if configTree.Tree.Config == nil {
		t.Error("Expected config to be parsed at root level")
	}

	// Should have children since python_lib has nested structure
	if len(configTree.Tree.Children) == 0 {
		t.Error("Expected children in python_lib structure")
	}

	// Should have src as a child
	if configTree.Tree.Children["src"] == nil {
		t.Error("Expected src directory in python_lib")
	}
}

func TestDiscoverConfigsFilesystemMode(t *testing.T) {
	// Test discovery outside git repository (filesystem mode)
	// Create a temporary directory structure
	tempDir := t.TempDir()

	// Create directory structure: lib/src/core and lib/src/utils/helpers
	err := os.MkdirAll(filepath.Join(tempDir, "lib", "src", "core"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}
	err = os.MkdirAll(filepath.Join(tempDir, "lib", "src", "utils", "helpers"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create config files at different levels
	configs := map[string]string{
		"lib/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: test-root
        name: Root Test`,
		"lib/src/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: test-src
        name: Src Test`,
		"lib/src/core/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: test-core
        name: Core Test`,
		"lib/src/utils/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: test-utils
        name: Utils Test`,
		"lib/src/utils/helpers/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: test-helpers
        name: Helpers Test`,
	}

	for path, content := range configs {
		fullPath := filepath.Join(tempDir, path)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file %s: %v", path, err)
		}
	}

	// Test discovery in the temp directory (not a git repo)
	libPath := filepath.Join(tempDir, "lib")
	configTree, err := DiscoverConfigs(libPath)
	if err != nil {
		t.Fatalf("Failed to discover configs in filesystem mode: %v", err)
	}

	// Should still work and find configs
	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node in filesystem mode")
	}

	if configTree.Tree.Config == nil {
		t.Error("Expected config to be parsed at root level in filesystem mode")
	}

	// Should have src as a child
	if configTree.Tree.Children["src"] == nil {
		t.Error("Expected src directory in filesystem mode")
	}

	// Verify nested structure
	srcNode := configTree.Tree.Children["src"]
	if srcNode.Children["core"] == nil {
		t.Error("Expected core directory in filesystem mode")
	}
	if srcNode.Children["utils"] == nil {
		t.Error("Expected utils directory in filesystem mode")
	}

	// Verify helpers
	utilsNode := srcNode.Children["utils"]
	if utilsNode.Children["helpers"] == nil {
		t.Error("Expected helpers directory in filesystem mode")
	}

	t.Logf("Filesystem mode tree:\n%s", configTree.String())
}

func TestDiscoverConfigsEmptyDirectory(t *testing.T) {
	// Test walking a directory with no configs (use a subdirectory without configs)
	emptyDir := "example_repos/example_monorepo/frontend/src"
	os.MkdirAll(emptyDir, 0755)

	configTree, err := DiscoverConfigs(emptyDir)
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Should find no tree structure or an empty root
	if configTree.Tree != nil && configTree.Tree.Config != nil {
		t.Error("Expected no config at root level")
	}

	if configTree.Tree != nil && len(configTree.Tree.Children) != 0 {
		t.Errorf("Expected no children, got %d", len(configTree.Tree.Children))
	}
}

func TestDiscoverConfigsNonExistentDirectory(t *testing.T) {
	// Test walking a non-existent directory
	_, err := DiscoverConfigs("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent directory")
	}
}

func TestConfigNodeParsing(t *testing.T) {
	// Test that configs are properly parsed with viper
	configTree, err := DiscoverConfigs("example_repos/example_monorepo/python_lib")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Should have root config and nested configs
	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node")
	}

	// Verify root config is parsed
	if configTree.Tree.Config == nil {
		t.Error("Root config should be parsed")
	}

	// Verify we can access config values
	repos := configTree.Tree.Config.Get("repos")
	if repos == nil {
		t.Error("Expected 'repos' key in root config")
	}

	// Verify nested config parsing
	srcNode := configTree.Tree.Children["src"]
	if srcNode == nil {
		t.Fatal("Expected src config node")
	}

	if srcNode.Config == nil {
		t.Error("Src config should be parsed")
	}

	srcRepos := srcNode.Config.Get("repos")
	if srcRepos == nil {
		t.Error("Expected 'repos' key in src config")
	}
}

func TestConfigTreeAbsolutePaths(t *testing.T) {
	// Test that absolute paths work correctly
	absPath, err := filepath.Abs("example_repos/example_monorepo")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	configTree, err := DiscoverConfigs(absPath)
	if err != nil {
		t.Fatalf("Failed to discover configs with absolute path: %v", err)
	}

	// Should still find the same tree structure
	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node")
	}

	// Should have the same main directories
	expectedMainDirs := []string{"python_lib", "go_lib", "frontend", "docs", "scripts"}
	for _, dir := range expectedMainDirs {
		if configTree.Tree.Children[dir] == nil {
			t.Errorf("Expected to find config in directory: %s", dir)
		}
	}
}

func TestConfigTreeRelativePaths(t *testing.T) {
	// Test that relative paths in results are correct
	configTree, err := DiscoverConfigs("example_repos/example_monorepo")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Helper function to check all nodes in tree
	var checkNode func(node *ConfigNode, currentPath string)
	checkNode = func(node *ConfigNode, currentPath string) {
		if node == nil {
			return
		}

		// If node has a config, path should be relative and end with config file name
		if node.Config != nil {
			if filepath.IsAbs(node.Path) {
				t.Errorf("Expected relative path, got absolute: %s", node.Path)
			}

			if filepath.Base(node.Path) != ".tree-commit-config.yaml" {
				t.Errorf("Expected config file name, got: %s", filepath.Base(node.Path))
			}
		}

		// Check children
		for childName, childNode := range node.Children {
			childPath := filepath.Join(currentPath, childName)
			checkNode(childNode, childPath)
		}
	}

	if configTree.Tree != nil {
		checkNode(configTree.Tree, "")
	}
}

func TestConfigTreePrettyPrint(t *testing.T) {
	// Test that the tree structure prints nicely
	configTree, err := DiscoverConfigs("example_repos/example_monorepo")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	// Test String() method
	treeString := configTree.String()
	if !strings.Contains(treeString, "ConfigTree(Root: example_repos/example_monorepo)") {
		t.Error("Expected root directory in string representation")
	}

	// Test PrettyPrint() method
	prettyOutput := configTree.PrettyPrint()

	// Should contain tree structure symbols
	if !strings.Contains(prettyOutput, "├──") && !strings.Contains(prettyOutput, "└──") {
		t.Error("Expected tree structure symbols in pretty print")
	}

	// Should contain file and folder emojis
	if !strings.Contains(prettyOutput, "📄") || !strings.Contains(prettyOutput, "📁") {
		t.Error("Expected file and folder emojis in pretty print")
	}

	// Should show config file names
	if !strings.Contains(prettyOutput, ".tree-commit-config.yaml") {
		t.Error("Expected config file names in pretty print")
	}

	// Should show directory structure
	expectedDirs := []string{"python_lib", "go_lib", "frontend", "docs", "scripts"}
	for _, dir := range expectedDirs {
		if !strings.Contains(prettyOutput, dir) {
			t.Errorf("Expected directory %s in pretty print", dir)
		}
	}

	// Print to console for manual verification (when running tests manually)
	t.Logf("Config tree structure:\n%s", configTree.String())
}

func TestConfigNodeString(t *testing.T) {
	// Test individual ConfigNode string representation
	configTree, err := DiscoverConfigs("example_repos/example_monorepo/python_lib")
	if err != nil {
		t.Fatalf("Failed to discover configs: %v", err)
	}

	if configTree.Tree == nil {
		t.Fatal("Expected root config tree node")
	}

	// Test root node string
	rootString := configTree.Tree.String()
	if !strings.Contains(rootString, "Config:") {
		t.Error("Expected 'Config:' in root node string")
	}

	if !strings.Contains(rootString, "Children:") {
		t.Error("Expected 'Children:' in root node string since it has children")
	}

	// Test child node string
	srcNode := configTree.Tree.Children["src"]
	if srcNode != nil {
		srcString := srcNode.String()
		if !strings.Contains(srcString, "Config:") {
			t.Error("Expected 'Config:' in src node string")
		}

		t.Logf("Src node string: %s", srcString)
	}
}
