package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGeminiTurns_MergeConsecutiveSameRoles(t *testing.T) {
	input := []byte(`{
		"contents": [
			{"role":"user","parts":[{"text":"first question"}]},
			{"role":"model","parts":[{"text":"first thought"}]},
			{"role":"model","parts":[{"functionCall":{"name":"read","args":{"file":"main.go"}}}]}
		]
	}`)

	out, err := normalizeGeminiTurns(input)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(out, &payload)
	require.NoError(t, err)

	contents, ok := payload["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 2, "Consecutive model turns should be merged into one")

	first, ok := contents[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", first["role"])

	second, ok := contents[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "model", second["role"])
	parts, ok := second["parts"].([]any)
	require.True(t, ok)
	assert.Len(t, parts, 2, "Merged turn should contain both parts")
}

func TestNormalizeGeminiTurns_PrependUserIfStartingWithFunctionCall(t *testing.T) {
	input := []byte(`{
		"contents": [
			{"role":"model","parts":[{"functionCall":{"name":"bash","args":{"cmd":"ls"}}}]}
		]
	}`)

	out, err := normalizeGeminiTurns(input)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(out, &payload)
	require.NoError(t, err)

	contents, ok := payload["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 2)
	first, ok := contents[0].(map[string]any)
	require.True(t, ok)
	second, ok := contents[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", first["role"])
	assert.Equal(t, "model", second["role"])
}
