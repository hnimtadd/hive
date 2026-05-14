package storage_test

import (
	"testing"

	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestSessionStorage_CreateAndLoad(t *testing.T) {
	t.Parallel()

	store, err := storage.NewSessionStorage(storage.Options{
		Storage: t.TempDir(),
	})
	require.NoError(t, err)

	session := &types.Session{
		ID: "session-001",
		Messages: []types.Message{
			types.NewMessage(types.RoleUser, "hello"),
		},
	}

	require.NoError(t, store.Create(session))

	loaded, err := store.Load("session-001")
	require.NoError(t, err)
	require.Equal(t, "session-001", loaded.ID)
	require.Len(t, loaded.Messages, 1)
	require.Equal(t, types.RoleUser, loaded.Messages[0].Role)
	require.Equal(t, "hello", loaded.Messages[0].Content)
}

func TestSessionStorage_SaveAppendsAndLoadReturnsLastSnapshot(t *testing.T) {
	t.Parallel()

	store, err := storage.NewSessionStorage(storage.Options{
		Storage: t.TempDir(),
	})
	require.NoError(t, err)

	session := &types.Session{
		ID: "session-append",
		Messages: []types.Message{
			types.NewMessage(types.RoleUser, "first"),
		},
	}

	require.NoError(t, store.Create(session))

	session.Messages = append(session.Messages, types.NewMessage(types.RoleAssistant, "second"))
	require.NoError(t, store.Save(session))

	loaded, err := store.Load("session-append")
	require.NoError(t, err)
	require.Len(t, loaded.Messages, 2)
	require.Equal(t, "second", loaded.Messages[1].Content)
}

func TestSessionStorage_List(t *testing.T) {
	t.Parallel()

	store, err := storage.NewSessionStorage(storage.Options{
		Storage: t.TempDir(),
	})
	require.NoError(t, err)

	require.NoError(t, store.Create(&types.Session{ID: "s1"}))
	require.NoError(t, store.Create(&types.Session{ID: "s2"}))

	sessions, err := store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 2)
}

func TestSessionStorage_RejectsInvalidSessionID(t *testing.T) {
	t.Parallel()

	store, err := storage.NewSessionStorage(storage.Options{
		Storage: t.TempDir(),
	})
	require.NoError(t, err)

	err = store.Save(&types.Session{
		ID: "../escape",
	})
	require.Error(t, err)
}
