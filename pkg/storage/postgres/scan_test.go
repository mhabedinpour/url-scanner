package postgres_test

import (
	"context"
	"scanner/pkg/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPgSQL_StoreScans(t *testing.T) {
	t.Parallel()

	pgSQL, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	userID := domain.UserID(uuid.New())
	URL1 := "https://google.com"
	URL2 := "https://yahoo.com"

	t.Run("store single scan", func(t *testing.T) {
		t.Parallel()

		s := domain.Scan{
			UserID: userID,
			URL:    URL1,
			Status: domain.ScanStatusPending,
		}

		res, err := pgSQL.StoreScans(ctx, s)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.Equal(t, URL1, res[0].URL)
	})

	t.Run("store multiple scans", func(t *testing.T) {
		t.Parallel()

		s1 := domain.Scan{
			UserID: userID,
			URL:    URL1,
			Status: domain.ScanStatusPending,
		}
		s2 := domain.Scan{
			UserID: userID,
			URL:    URL2,
			Status: domain.ScanStatusPending,
		}

		res, err := pgSQL.StoreScans(ctx, s1, s2)
		require.NoError(t, err)
		require.Len(t, res, 2)
	})

	t.Run("store empty scans", func(t *testing.T) {
		t.Parallel()

		res, err := pgSQL.StoreScans(ctx)
		require.NoError(t, err)
		require.Empty(t, res)
	})
}

func TestPgSQL_UpdatePendingScansByURL(t *testing.T) {
	t.Parallel()

	pgSQL, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	userID := domain.UserID(uuid.New())
	urlA := "https://example.com/a"
	urlB := "https://example.com/b"

	// insert scans
	s1 := domain.Scan{UserID: userID, URL: urlA, Status: domain.ScanStatusPending}
	s2 := domain.Scan{UserID: userID, URL: urlA, Status: domain.ScanStatusPending}
	s3 := domain.Scan{UserID: userID, URL: urlA, Status: domain.ScanStatusCompleted}
	s4 := domain.Scan{UserID: userID, URL: urlB, Status: domain.ScanStatusPending}
	ins, err := pgSQL.StoreScans(ctx, s1, s2, s3, s4)
	require.NoError(t, err)
	require.Len(t, ins, 4)

	// update only pending scans for urlA
	empty := ""
	u := struct {
		Status    domain.ScanStatus
		Result    *domain.ScanResult
		LastError *string
	}{
		Status:    domain.ScanStatusCompleted,
		Result:    &domain.ScanResult{},
		LastError: &empty, // clear last_error to NULL
	}
	require.NoError(t, pgSQL.UpdatePendingScansByURL(ctx, urlA, u))

	// fetch all user scans and validate
	page, err := pgSQL.UserScans(ctx, userID, time.Time{}, 50)
	require.NoError(t, err)

	// build index by id
	byID := map[uuid.UUID]domain.Scan{}
	for _, sc := range page.Scans {
		byID[uuid.UUID(sc.ID)] = sc
	}

	// assertions for s1, s2 updated
	for i := range 2 {
		sc := byID[uuid.UUID(ins[i].ID)]
		require.Equal(t, domain.ScanStatusCompleted, sc.Status)
		require.EqualValues(t, 1, sc.Attempts)
		require.False(t, sc.UpdatedAt.IsZero())
		require.Empty(t, sc.LastError)
	}
	// s3 (completed) should remain with attempts 0
	sc3 := byID[uuid.UUID(ins[2].ID)]
	require.EqualValues(t, 0, sc3.Attempts)
	// s4 for urlB should remain pending
	sc4 := byID[uuid.UUID(ins[3].ID)]
	require.Equal(t, domain.ScanStatusPending, sc4.Status)
}

func TestPgSQL_DeleteScan(t *testing.T) {
	t.Parallel()

	pgSQL, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	userID := domain.UserID(uuid.New())
	s := domain.Scan{UserID: userID, URL: "https://delete.me", Status: domain.ScanStatusPending}
	stored, err := pgSQL.StoreScans(ctx, s)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	id := stored[0].ID

	// delete
	deleted, err := pgSQL.DeleteScan(ctx, userID, id)
	require.NoError(t, err)
	require.NotNil(t, deleted)
	require.Equal(t, id, deleted.ID)
	// fetching by id should return nil
	got, err := pgSQL.ScanByID(ctx, userID, id)
	require.NoError(t, err)
	require.Nil(t, got)
	// listing should not include it
	page, err := pgSQL.UserScans(ctx, userID, time.Time{}, 10)
	require.NoError(t, err)
	for _, sc := range page.Scans {
		require.NotEqual(t, id, sc.ID)
	}
	// deleting again should not error
	deleted2, err := pgSQL.DeleteScan(ctx, userID, id)
	require.NoError(t, err)
	require.Nil(t, deleted2)
}

func TestPgSQL_UserScans_Pagination(t *testing.T) {
	t.Parallel()

	pgSQL, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	userID := domain.UserID(uuid.New())
	// insert 5 scans
	scans := make([]domain.Scan, 0, 5)
	for range 5 {
		sc := domain.Scan{UserID: userID, URL: "https://page.example/" + uuid.NewString(), Status: domain.ScanStatusPending}
		scans = append(scans, sc)
	}
	stored, err := pgSQL.StoreScans(ctx, scans...)
	require.NoError(t, err)
	require.Len(t, stored, 5)

	// adjust created_at to be deterministic descending: now, now-1m, ...
	now := time.Now().UTC()
	for i, sc := range stored {
		created := now.Add(-time.Duration(4-i) * time.Minute) // stored order is same as input; make last newest
		_, err := pgSQL.DB.ExecContext(ctx, "UPDATE scans SET created_at = $1 WHERE id = $2", created, uuid.UUID(sc.ID))
		require.NoError(t, err)
	}

	// first page, limit 2
	p1, err := pgSQL.UserScans(ctx, userID, time.Time{}, 2)
	require.NoError(t, err)
	require.Len(t, p1.Scans, 2)
	require.NotNil(t, p1.NextCursor)
	c1 := *p1.NextCursor

	// second page
	p2, err := pgSQL.UserScans(ctx, userID, c1, 2)
	require.NoError(t, err)
	require.Len(t, p2.Scans, 2)
	require.NotNil(t, p2.NextCursor)
	c2 := *p2.NextCursor

	// third (last) page, should have 1 left and no next cursor
	p3, err := pgSQL.UserScans(ctx, userID, c2, 2)
	require.NoError(t, err)
	require.Len(t, p3.Scans, 1)
	require.Nil(t, p3.NextCursor)
}

func TestPgSQL_ScanByID(t *testing.T) {
	t.Parallel()

	pgSQL, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	userA := domain.UserID(uuid.New())
	userB := domain.UserID(uuid.New())
	storedA, err := pgSQL.StoreScans(ctx, domain.Scan{
		UserID: userA,
		URL:    "https://id.test/a",
		Status: domain.ScanStatusPending,
	})
	require.NoError(t, err)
	storedB, err := pgSQL.StoreScans(ctx, domain.Scan{UserID: userB,
		URL:    "https://id.test/b",
		Status: domain.ScanStatusPending,
	})
	require.NoError(t, err)
	idA := storedA[0].ID
	idB := storedB[0].ID

	// correct user & id
	got, err := pgSQL.ScanByID(ctx, userA, idA)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, idA, got.ID)

	// wrong user should not see other's scan
	got2, err := pgSQL.ScanByID(ctx, userA, idB)
	require.NoError(t, err)
	require.Nil(t, got2)

	// soft delete and ensure not returned
	_, err = pgSQL.DeleteScan(ctx, userA, idA)
	require.NoError(t, err)
	got3, err := pgSQL.ScanByID(ctx, userA, idA)
	require.NoError(t, err)
	require.Nil(t, got3)
}
