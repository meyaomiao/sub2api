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

	contents := payload["contents"].([]any)
	require.Len(t, contents, 2, "Consecutive model turns should be merged into one")

	first := contents[0].(map[string]any)
	assert.Equal(t, "user", first["role"])

	second := contents[1].(map[string]any)
	assert.Equal(t, "model", second["role"])
	parts := second["parts"].([]any)
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

	contents := payload["contents"].([]any)
	require.Len(t, contents, 2)
	assert.Equal(t, "user", contents[0].(map[string]any)["role"])
	assert.Equal(t, "model", contents[1].(map[string]any)["role"])
}
