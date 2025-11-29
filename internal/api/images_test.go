package api

import (
	"encoding/json"
	"testing"

	"github.com/runvoy/runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterImageRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with is_default", func(t *testing.T) {
		isDefault := true
		req := RegisterImageRequest{
			Image:     "alpine:latest",
			IsDefault: &isDefault,
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled RegisterImageRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Image, unmarshaled.Image)
		assert.NotNil(t, unmarshaled.IsDefault)
		assert.Equal(t, *req.IsDefault, *unmarshaled.IsDefault)
	})

	t.Run("marshal and unmarshal without is_default", func(t *testing.T) {
		req := RegisterImageRequest{
			Image: "alpine:latest",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled RegisterImageRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Image, unmarshaled.Image)
		assert.Nil(t, unmarshaled.IsDefault)
	})
}

func TestRegisterImageResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := RegisterImageResponse{
			Image:   "alpine:latest",
			Message: "Image registered successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled RegisterImageResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Image, unmarshaled.Image)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestRemoveImageRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		req := RemoveImageRequest{
			Image: "alpine:latest",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled RemoveImageRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Image, unmarshaled.Image)
	})
}

func TestRemoveImageResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := RemoveImageResponse{
			Image:   "alpine:latest",
			Message: "Image removed successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled RemoveImageResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Image, unmarshaled.Image)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestImageInfoJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		isDefault := true
		info := ImageInfo{
			Image:              "alpine:latest",
			TaskDefinitionName: constants.ProjectName,
			IsDefault:          &isDefault,
		}

		data, err := json.Marshal(info)
		require.NoError(t, err)

		var unmarshaled ImageInfo
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, info.Image, unmarshaled.Image)
		assert.Equal(t, info.TaskDefinitionName, unmarshaled.TaskDefinitionName)
		assert.NotNil(t, unmarshaled.IsDefault)
		assert.Equal(t, *info.IsDefault, *unmarshaled.IsDefault)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		info := ImageInfo{
			Image: "alpine:latest",
		}

		data, err := json.Marshal(info)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "alpine:latest")
		assert.NotContains(t, jsonStr, "task_definition_arn")
		assert.NotContains(t, jsonStr, "is_default")
	})
}

func TestListImagesResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		isDefault := true
		resp := ListImagesResponse{
			Images: []ImageInfo{
				{
					Image:     "alpine:latest",
					IsDefault: &isDefault,
				},
				{
					Image: "ubuntu:20.04",
				},
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ListImagesResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Images, 2)
		assert.Equal(t, resp.Images[0].Image, unmarshaled.Images[0].Image)
		assert.NotNil(t, unmarshaled.Images[0].IsDefault)
		assert.Nil(t, unmarshaled.Images[1].IsDefault)
	})

	t.Run("empty list", func(t *testing.T) {
		resp := ListImagesResponse{
			Images: []ImageInfo{},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ListImagesResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Empty(t, unmarshaled.Images)
	})
}
