package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPreCommitHook(t *testing.T) {
	// Create a temporary directory with a fake git repository
	tempDir := t.TempDir()

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create .git directory to simulate git repository
	gitDir := ".git"
	err = os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Test the install function
	err = installPreCommitHook()
	if err != nil {
		t.Fatalf("installPreCommitHook failed: %v", err)
	}

	// Verify hooks directory was created
	hooksDir := filepath.Join(gitDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Hooks directory was not created")
	}

	// Verify hook file was created
	hookPath := filepath.Join(hooksDir, "tree-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Hook file was not created")
	}

	// Verify hook file content
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook file: %v", err)
	}

	hookContent := string(content)
	if !strings.HasPrefix(hookContent, "#!/bin/sh\n") {
		t.Error("Hook file should start with shebang")
	}

	if !strings.Contains(hookContent, "run") {
		t.Error("Hook file should contain 'run' command")
	}

	// Verify file permissions (should be executable)
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Failed to stat hook file: %v", err)
	}

	mode := info.Mode()
	if mode&0111 == 0 {
		t.Error("Hook file should be executable")
	}
}

func TestInstallPreCommitHookNotInGitRepo(t *testing.T) {
	// Create a temporary directory without .git
	tempDir := t.TempDir()

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test the install function - should fail
	err = installPreCommitHook()
	if err == nil {
		t.Error("Expected error when not in git repository")
	}

	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("Expected 'not in a git repository' error, got: %v", err)
	}
}

func TestInstallPreCommitHookExistingHooksDir(t *testing.T) {
	// Create a temporary directory with existing .git/hooks
	tempDir := t.TempDir()

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create .git and hooks directories
	gitDir := ".git"
	hooksDir := filepath.Join(gitDir, "hooks")
	err = os.MkdirAll(hooksDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create hooks directory: %v", err)
	}

	// Create an existing file in hooks directory
	existingFile := filepath.Join(hooksDir, "pre-commit")
	err = os.WriteFile(existingFile, []byte("existing content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Test the install function
	err = installPreCommitHook()
	if err != nil {
		t.Fatalf("installPreCommitHook failed: %v", err)
	}

	// Verify existing file is still there
	if _, err := os.Stat(existingFile); os.IsNotExist(err) {
		t.Error("Existing file should not be removed")
	}

	// Verify our hook file was created
	hookPath := filepath.Join(hooksDir, "tree-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Hook file was not created")
	}
}

func TestInstallPreCommitHookOverwriteExisting(t *testing.T) {
	// Test that installing overwrites existing tree-commit hook
	tempDir := t.TempDir()

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create .git and hooks directories
	gitDir := ".git"
	hooksDir := filepath.Join(gitDir, "hooks")
	err = os.MkdirAll(hooksDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create hooks directory: %v", err)
	}

	// Create existing tree-commit hook
	hookPath := filepath.Join(hooksDir, "tree-commit")
	oldContent := "#!/bin/sh\necho 'old hook'"
	err = os.WriteFile(hookPath, []byte(oldContent), 0755)
	if err != nil {
		t.Fatalf("Failed to create existing hook: %v", err)
	}

	// Test the install function
	err = installPreCommitHook()
	if err != nil {
		t.Fatalf("installPreCommitHook failed: %v", err)
	}

	// Verify hook file was overwritten
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook file: %v", err)
	}

	newContent := string(content)
	if newContent == oldContent {
		t.Error("Hook file should have been overwritten")
	}

	if !strings.Contains(newContent, "run") {
		t.Error("New hook file should contain 'run' command")
	}
}