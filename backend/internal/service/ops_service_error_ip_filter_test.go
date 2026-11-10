package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUserErrorRequests_IPFilterCannotOverrideUserScope(t *testing.T) {
	repo := &stubOpsRepoForUserErr{}
	svc := &OpsService{opsRepo: repo}
	otherUser, keyID := int64(99), int64(7)
	filter := &OpsErrorLogFilter{UserID: &otherUser, APIKeyID: &keyID, IPAddresses: []string{"192.0.2.1", "2001:db8::1"}}
	_, err := svc.ListUserErrorRequests(context.Background(), 42, filter)
	require.NoError(t, err)
	require.NotNil(t, repo.gotFilter)
	require.NotNil(t, repo.gotFilter.UserID)
	assert.EqualValues(t, 42, *repo.gotFilter.UserID)
	assert.Equal(t, filter.IPAddresses, repo.gotFilter.IPAddresses)
	assert.Equal(t, filter.APIKeyID, repo.gotFilter.APIKeyID)
	assert.EqualValues(t, 99, *filter.UserID, "caller filter must not be mutated")
}
