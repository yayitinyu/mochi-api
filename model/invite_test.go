package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInviteRedeemIsAtomic(t *testing.T) {
	setupTestDB(t)

	codes, err := CreateInviteCodes(1, 1)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	code := codes[0].Code

	first := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUserWithInvite(first, code))

	// A second redemption of the same code must fail and roll back the
	// user it tried to create.
	second := &User{Username: "bob", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	err = CreateUserWithInvite(second, code)
	require.ErrorIs(t, err, ErrInviteInvalid)

	count, err := CountUsers()
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func TestRedeemUnknownCodeCreatesNoUser(t *testing.T) {
	setupTestDB(t)

	user := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	err := CreateUserWithInvite(user, "no-such-code")
	require.ErrorIs(t, err, ErrInviteInvalid)

	count, err := CountUsers()
	require.NoError(t, err)
	require.EqualValues(t, 0, count)
}

func TestDeleteUsedInviteCodeRefused(t *testing.T) {
	setupTestDB(t)

	codes, err := CreateInviteCodes(1, 2)
	require.NoError(t, err)

	user := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUserWithInvite(user, codes[0].Code))

	require.ErrorIs(t, DeleteInviteCode(codes[0].Id), gorm.ErrRecordNotFound)
	require.NoError(t, DeleteInviteCode(codes[1].Id))
}

func TestListInviteCodesIncludesRedeemerUsername(t *testing.T) {
	setupTestDB(t)

	empty, err := ListInviteCodes()
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)

	codes, err := CreateInviteCodes(1, 2)
	require.NoError(t, err)

	user := &User{Username: "alice", Password: "hash", Role: RoleUser, Status: StatusEnabled}
	require.NoError(t, CreateUserWithInvite(user, codes[0].Code))

	list, err := ListInviteCodes()
	require.NoError(t, err)
	require.Len(t, list, 2)
	byId := map[int]InviteCodeView{}
	for _, v := range list {
		byId[v.Id] = v
	}
	require.Equal(t, "alice", byId[codes[0].Id].UsedByUsername)
	require.Equal(t, user.Id, byId[codes[0].Id].UsedByUserId)
	require.Equal(t, "", byId[codes[1].Id].UsedByUsername)
	require.Equal(t, 0, byId[codes[1].Id].UsedByUserId)
}
