package antigravity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingProcessor_OnlyThoughtSignatureMarksContent(t *testing.T) {
	proc := NewStreamingProcessor("gemini-3.8-flash-high")
	require.False(t, proc.HasContent())

	// Simulate upstream chunk containing only thoughtSignature
	chunk := `data: {"response":{"candidates":[{"content":{"role":"model","parts":[{"thoughtSignature":"dGVzdA=="}]},"finishReason":""}]}}`
	output := proc.ProcessLine(chunk)
	assert.NotEmpty(t, output)
	assert.True(t, proc.HasContent(), "Processor must recognize thoughtSignature as substantive content to prevent empty stream failover")

	// Finish should emit termination cleanly without dropping
	finalEvents, _ := proc.Finish()
	assert.NotEmpty(t, finalEvents)
}
