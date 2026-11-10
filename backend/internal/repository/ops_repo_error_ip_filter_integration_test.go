//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListErrorLogsIPAddressFilter(t *testing.T) {
	ctx := context.Background()
	db := integrationDB
	repo := NewOpsRepository(db)
	requestID := t.Name() + time.Now().Format(time.RFC3339Nano)
	var userID, otherUserID int64
	t.Cleanup(func() {
		_, err := db.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE request_id = $1", requestID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userID, otherUserID)
		require.NoError(t, err)
	})
	for i, id := range []*int64{&userID, &otherUserID} {
		err := db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'test') RETURNING id`,
			t.Name()+time.Now().Format(time.RFC3339Nano)+string(rune('a'+i))+"@example.com").Scan(id)
		require.NoError(t, err)
	}
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(ip any, owner int64, excluded bool, status int) int64 {
		t.Helper()
		var id int64
		err := db.QueryRowContext(ctx, `INSERT INTO ops_error_logs
			(created_at, error_phase, error_type, severity, status_code, client_ip, user_id, is_business_limited, request_id)
			VALUES ($1, 'request', 'api_error', 'error', $2, $3::inet, $4, $5, $6) RETURNING id`,
			createdAt, status, ip, owner, excluded, requestID).Scan(&id)
		require.NoError(t, err)
		return id
	}
	v4 := insert("192.0.2.1", userID, false, 500)
	v6 := insert("2001:0db8:0:0:0:0:0:1", userID, false, 502)
	excluded := insert("2001:db8::1", userID, true, 429)
	insert("192.0.2.2", userID, false, 500)
	insert(nil, userID, false, 500)
	insert("192.0.2.1", otherUserID, false, 500)
	insert("2001:db8::1", otherUserID, true, 429)
	insert("192.0.2.1", userID, false, 200)

	for _, tc := range []struct {
		name string
		view string
		ips  []string
		want []int64
	}{
		{name: "all", view: "all", ips: []string{"192.0.2.1", "2001:db8::1"}, want: []int64{excluded, v6, v4}},
		{name: "errors", view: "errors", ips: []string{"192.0.2.1", "2001:db8::1"}, want: []int64{v6, v4}},
		{name: "excluded", view: "excluded", ips: []string{"192.0.2.1", "2001:db8::1"}, want: []int64{excluded}},
		{name: "IPv6 equivalent spelling", view: "all", ips: []string{"2001:0db8:0000:0000:0000:0000:0000:0001"}, want: []int64{excluded, v6}},
		{name: "no match", view: "all", ips: []string{"192.0.2.99"}, want: []int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := &service.OpsErrorLogFilter{RequestID: requestID, UserID: &userID, View: tc.view, IPAddresses: tc.ips, Page: 1, PageSize: 1}
			got := make([]int64, 0, len(tc.want))
			for page := 1; page <= len(tc.want)+1; page++ {
				filter.Page = page
				list, err := repo.ListErrorLogs(ctx, filter)
				require.NoError(t, err)
				assert.Equal(t, len(tc.want), list.Total, "count must use the same IP and user filters as the list")
				assert.Equal(t, page, list.Page)
				assert.LessOrEqual(t, len(list.Errors), 1)
				for _, item := range list.Errors {
					got = append(got, item.ID)
					require.NotNil(t, item.UserID)
					assert.Equal(t, userID, *item.UserID)
				}
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
