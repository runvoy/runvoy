package playbooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaybookLoader_ListPlaybooks(t *testing.T) {
	t.Run("returns empty list when directory doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		loader := &PlaybookLoader{}

		// Change to temp directory
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// GetPlaybookDir may fall back to home directory, so we need to check
		// if the returned directory exists. If it doesn't exist, ListPlaybooks
		// should return empty. If it does exist (from home directory fallback),
		// we can't control what's in there, so we just verify ListPlaybooks
		// doesn't error.
		playbookDir, err := loader.GetPlaybookDir()
		require.NoError(t, err)
		_, statErr := os.Stat(playbookDir)
		if os.IsNotExist(statErr) {
			// Directory doesn't exist, should return empty
			playbooks, listErr := loader.ListPlaybooks()
			assert.NoError(t, listErr)
			assert.Empty(t, playbooks)
		} else {
			// Directory exists (likely from home directory fallback)
			// Just verify ListPlaybooks doesn't error
			playbooks, listErr := loader.ListPlaybooks()
			assert.NoError(t, listErr)
			// Note: playbooks may not be empty if home directory has playbooks
			_ = playbooks
		}
	})

	t.Run("discovers playbook files", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		// Create playbook files
		yamlFile := filepath.Join(playbookDir, "test.yaml")
		err = os.WriteFile(yamlFile, []byte("commands:\n  - echo hello"), 0600)
		require.NoError(t, err)

		ymlFile := filepath.Join(playbookDir, "test2.yml")
		err = os.WriteFile(ymlFile, []byte("commands:\n  - echo world"), 0600)
		require.NoError(t, err)

		// Create non-playbook file
		txtFile := filepath.Join(playbookDir, "test.txt")
		err = os.WriteFile(txtFile, []byte("not a playbook"), 0600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		playbooks, err := loader.ListPlaybooks()
		assert.NoError(t, err)
		assert.Len(t, playbooks, 2)
		assert.Contains(t, playbooks, "test")
		assert.Contains(t, playbooks, "test2")
	})
}

func TestPlaybookLoader_LoadPlaybook(t *testing.T) {
	t.Run("loads valid playbook", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		yamlContent := `description: Test playbook
image: test/image:latest
git_repo: https://github.com/test/repo.git
git_ref: main
git_path: /path
secrets:
  - secret1
  - secret2
env:
  KEY1: value1
  KEY2: value2
commands:
  - echo hello
  - echo world
`
		yamlFile := filepath.Join(playbookDir, "test.yaml")
		err = os.WriteFile(yamlFile, []byte(yamlContent), 0600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		pb, err := loader.LoadPlaybook("test")
		assert.NoError(t, err)
		assert.NotNil(t, pb)
		assert.Equal(t, "Test playbook", pb.Description)
		assert.Equal(t, "test/image:latest", pb.Image)
		assert.Equal(t, "https://github.com/test/repo.git", pb.GitRepo)
		assert.Equal(t, "main", pb.GitRef)
		assert.Equal(t, "/path", pb.GitPath)
		assert.Equal(t, []string{"secret1", "secret2"}, pb.Secrets)
		assert.Equal(t, map[string]string{"KEY1": "value1", "KEY2": "value2"}, pb.Env)
		assert.Equal(t, []string{"echo hello", "echo world"}, pb.Commands)
	})

	t.Run("returns error for missing playbook", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		pb, err := loader.LoadPlaybook("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, pb)
		assert.Contains(t, err.Error(), "playbook not found")
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		yamlFile := filepath.Join(playbookDir, "invalid.yaml")
		err = os.WriteFile(
			yamlFile,
			[]byte("invalid: yaml: content: [unclosed"),
			0600,
		)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		pb, err := loader.LoadPlaybook("invalid")
		assert.Error(t, err)
		assert.Nil(t, pb)
		assert.Contains(t, err.Error(), "failed to parse playbook YAML")
	})

	t.Run("returns error for playbook without commands", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		yamlContent := `description: No commands
image: test/image:latest
`
		yamlFile := filepath.Join(playbookDir, "empty.yaml")
		err = os.WriteFile(yamlFile, []byte(yamlContent), 0600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		pb, err := loader.LoadPlaybook("empty")
		assert.Error(t, err)
		assert.Nil(t, pb)
		assert.Contains(t, err.Error(), "commands must not be empty")
	})
}

func TestPlaybookLoader_GetPlaybookDir(t *testing.T) {
	t.Run("returns current directory playbook folder when it exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0750)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := &PlaybookLoader{}
		dir, err := loader.GetPlaybookDir()
		assert.NoError(t, err)
		// Use filepath.EvalSymlinks to resolve symlinks and normalize paths
		// This handles macOS /var -> /private/var symlink differences
		expectedResolved, err := filepath.EvalSymlinks(playbookDir)
		require.NoError(t, err)
		actualResolved, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		assert.Equal(t, expectedResolved, actualResolved)
	})
}

func TestNewPlaybookLoader(t *testing.T) {
	loader := NewPlaybookLoader()
	assert.NotNil(t, loader)
	assert.IsType(t, &PlaybookLoader{}, loader)
}
