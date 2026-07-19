//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTGShopService_QueryBalance(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 7, Email: "u@example.com", Balance: 12.34, FrozenBalance: 5.6}}
	svc := NewTGShopService(repo, nil, nil, nil)

	// 邮箱大小写/空白规范化后命中。
	balance, frozen, err := svc.QueryBalance(context.Background(), "  U@Example.com ")
	require.NoError(t, err)
	require.Equal(t, 12.34, balance)
	require.Equal(t, 5.6, frozen)
}

func TestTGShopService_QueryBalance_EmptyEmail(t *testing.T) {
	svc := NewTGShopService(&userRepoStub{}, nil, nil, nil)
	_, _, err := svc.QueryBalance(context.Background(), "   ")
	require.Error(t, err)
}

func TestTGShopService_QueryBalance_UserNotFound(t *testing.T) {
	repo := &userRepoStub{getByEmailErr: ErrUserNotFound}
	svc := NewTGShopService(repo, nil, nil, nil)
	_, _, err := svc.QueryBalance(context.Background(), "missing@example.com")
	require.Error(t, err)
}
