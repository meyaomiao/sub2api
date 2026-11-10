package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type opsErrorIPFilterRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
}

func (r *opsErrorIPFilterRepo) ListErrorLogs(_ context.Context, filter *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	r.filter = filter
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{}, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func TestGetErrorLogsIPAddressFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, view := range []string{"errors", "excluded", "all"} {
		for _, tc := range []struct {
			name    string
			raw     string
			want    []string
			invalid bool
		}{
			{name: "omitted"},
			{name: "empty segments", raw: " , , "},
			{name: "IPv4 and IPv6", raw: " 192.0.2.1, ,2001:0db8:0000:0000:0000:0000:0000:0001,", want: []string{"192.0.2.1", "2001:db8::1"}},
			{name: "invalid among valid", raw: "192.0.2.1,invalid,2001:db8::1", invalid: true},
			{name: "CIDR is not an address", raw: "192.0.2.0/24", invalid: true},
			{name: "too many addresses", raw: strings.Repeat("192.0.2.1,", 33), invalid: true},
		} {
			t.Run(view+"/"+tc.name, func(t *testing.T) {
				repo := &opsErrorIPFilterRepo{}
				h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				params := url.Values{"view": {view}, "ip_address": {tc.raw}, "user_id": {"42"}, "api_key_id": {"7"}}
				c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/errors?"+params.Encode(), nil)
				h.GetErrorLogs(c)
				if tc.invalid {
					assert.Equal(t, http.StatusBadRequest, recorder.Code)
					assert.Contains(t, recorder.Body.String(), "ip_address")
					assert.Nil(t, repo.filter, "invalid filters must not reach the repository")
					return
				}
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.NotNil(t, repo.filter)
				assert.Equal(t, tc.want, repo.filter.IPAddresses)
				assert.Equal(t, view, repo.filter.View)
				require.NotNil(t, repo.filter.UserID)
				assert.EqualValues(t, 42, *repo.filter.UserID)
				require.NotNil(t, repo.filter.APIKeyID)
				assert.EqualValues(t, 7, *repo.filter.APIKeyID)
			})
		}
	}
}
