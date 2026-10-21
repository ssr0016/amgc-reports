package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

func TestMockReportRepo(t *testing.T) {
	ctx := context.Background()
	m := NewMockReportRepo()
	v := json.Number("120.5")
	req := model.CreateReportRequest{Month: "January", Worker: "Ana",
		Metrics: map[string]model.MetricWeeks{"outreach": {Week1: &v}}}

	r, err := m.Create(ctx, 1, 2026, req)
	require.NoError(t, err)
	assert.Equal(t, "120.5", string(*r.ToResponse().Metrics["outreach"].Week1))

	_, err = m.Create(ctx, 1, 2026, req)
	require.ErrorIs(t, err, ErrUniqueViolation)

	on, err := m.ToggleComplete(ctx, r.ID, 9)
	require.NoError(t, err)
	assert.True(t, on.Completed)

	list, total, err := m.List(ctx, model.ListReportsParams{Worker: "AN"}, pagination.Params{Page: 1, Limit: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Len(t, list, 1)

	require.NoError(t, m.SoftDelete(ctx, r.ID))
	got, err := m.Get(ctx, r.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
	require.ErrorIs(t, m.SoftDelete(ctx, r.ID), ErrNotFound)

	_, err = m.Create(ctx, 1, 2026, req) // period is free again
	require.NoError(t, err)
}
