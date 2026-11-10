package repository

import (
	"database/sql/driver"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOpsErrorLogsWhere_IPAddressesPreservesScope(t *testing.T) {
	for _, view := range []string{"errors", "excluded", "all"} {
		t.Run(view, func(t *testing.T) {
			uid, keyID := int64(42), int64(7)
			filter := &service.OpsErrorLogFilter{View: view, UserID: &uid, APIKeyID: &keyID,
				IPAddresses: []string{"192.0.2.1", "2001:db8::1"}}
			where, args := buildOpsErrorLogsWhere(filter)
			assert.Contains(t, where, "e.user_id = $1 AND e.api_key_id = $2 AND e.client_ip = ANY($3::inet[])")
			require.Len(t, args, 3)
			assert.Equal(t, uid, args[0])
			assert.Equal(t, keyID, args[1])
			valuer, ok := args[2].(driver.Valuer)
			require.True(t, ok)
			encoded, err := valuer.Value()
			require.NoError(t, err)
			assert.Equal(t, `{"192.0.2.1","2001:db8::1"}`, encoded)

			filter.IPAddresses = nil
			where, args = buildOpsErrorLogsWhere(filter)
			assert.NotContains(t, where, "e.client_ip")
			assert.Len(t, args, 2)
		})
	}
}
