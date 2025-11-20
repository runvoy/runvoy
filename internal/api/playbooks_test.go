package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPlaybookYAML(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		playbook := Playbook{
			Description: "Deploy application",
			Image:       "node:18",
			GitRepo:     "https://github.com/example/repo",
			GitRef:      "main",
			GitPath:     "/app",
			Secrets:     []string{"API_KEY", "DB_PASSWORD"},
			Env: map[string]string{
				"NODE_ENV": "production",
				"PORT":     "3000",
			},
			Commands: []string{
				"npm install",
				"npm run build",
				"npm start",
			},
		}

		data, err := yaml.Marshal(playbook)
		require.NoError(t, err)

		var unmarshaled Playbook
		err = yaml.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, playbook.Description, unmarshaled.Description)
		assert.Equal(t, playbook.Image, unmarshaled.Image)
		assert.Equal(t, playbook.GitRepo, unmarshaled.GitRepo)
		assert.Equal(t, playbook.GitRef, unmarshaled.GitRef)
		assert.Equal(t, playbook.GitPath, unmarshaled.GitPath)
		assert.Equal(t, playbook.Secrets, unmarshaled.Secrets)
		assert.Equal(t, playbook.Env, unmarshaled.Env)
		assert.Equal(t, playbook.Commands, unmarshaled.Commands)
	})

	t.Run("minimal playbook with only commands", func(t *testing.T) {
		playbook := Playbook{
			Commands: []string{"echo hello"},
		}

		data, err := yaml.Marshal(playbook)
		require.NoError(t, err)

		var unmarshaled Playbook
		err = yaml.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Empty(t, unmarshaled.Description)
		assert.Empty(t, unmarshaled.Image)
		assert.Equal(t, playbook.Commands, unmarshaled.Commands)
	})

	t.Run("omit empty fields in yaml", func(t *testing.T) {
		playbook := Playbook{
			Image:    "alpine:latest",
			Commands: []string{"ls -la"},
		}

		data, err := yaml.Marshal(playbook)
		require.NoError(t, err)

		yamlStr := string(data)
		assert.NotContains(t, yamlStr, "description")
		assert.NotContains(t, yamlStr, "git_repo")
		assert.NotContains(t, yamlStr, "git_ref")
		assert.NotContains(t, yamlStr, "secrets")
		assert.Contains(t, yamlStr, "image")
		assert.Contains(t, yamlStr, "commands")
	})

	t.Run("parse yaml with all fields", func(t *testing.T) {
		yamlData := `
description: Test playbook
image: python:3.11
git_repo: https://github.com/test/repo
git_ref: develop
git_path: /src
secrets:
  - SECRET_A
  - SECRET_B
env:
  DEBUG: "true"
  LOG_LEVEL: info
commands:
  - pip install -r requirements.txt
  - python main.py
`
		var playbook Playbook
		err := yaml.Unmarshal([]byte(yamlData), &playbook)
		require.NoError(t, err)

		assert.Equal(t, "Test playbook", playbook.Description)
		assert.Equal(t, "python:3.11", playbook.Image)
		assert.Equal(t, "https://github.com/test/repo", playbook.GitRepo)
		assert.Equal(t, "develop", playbook.GitRef)
		assert.Equal(t, "/src", playbook.GitPath)
		assert.Equal(t, []string{"SECRET_A", "SECRET_B"}, playbook.Secrets)
		assert.Equal(t, "true", playbook.Env["DEBUG"])
		assert.Equal(t, "info", playbook.Env["LOG_LEVEL"])
		assert.Len(t, playbook.Commands, 2)
	})
}

func TestPlaybookJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		playbook := Playbook{
			Description: "Build and test",
			Image:       "golang:1.21",
			Commands:    []string{"go build", "go test"},
		}

		data, err := json.Marshal(playbook)
		require.NoError(t, err)

		var unmarshaled Playbook
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, playbook.Description, unmarshaled.Description)
		assert.Equal(t, playbook.Image, unmarshaled.Image)
		assert.Equal(t, playbook.Commands, unmarshaled.Commands)
	})
}
