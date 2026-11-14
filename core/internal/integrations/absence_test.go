package integrations

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPolicyReturnsNotes(t *testing.T) {
	t.Parallel()

	service := NewMockAbsence(0)

	notes, err := service.Policy(context.Background(), AbsenceInput{
		DateFrom:    "2026-11-20",
		DateTo:      "2026-11-20",
		AbsenceType: "vacation",
	})
	require.NoError(t, err)
	require.Len(t, notes, 2)
	require.Contains(t, notes[0], "согласует руководитель")
}

func TestPolicyDependsOnAbsenceType(t *testing.T) {
	t.Parallel()

	service := NewMockAbsence(0)

	notes, err := service.Policy(context.Background(), AbsenceInput{AbsenceType: "sick_leave"})
	require.NoError(t, err)
	require.Contains(t, notes[1], "больничный")

	notes, err = service.Policy(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.Contains(t, notes[1], "отчётность")
}

func TestCreateReturnsExternalID(t *testing.T) {
	t.Parallel()

	service := NewMockAbsence(0)

	result, err := service.Create(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.Equal(t, "ABS-1025", result.ExternalID)
	require.Equal(t, "pending", result.Status)

	next, err := service.Create(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.Equal(t, "ABS-1026", next.ExternalID)
}

func TestFailNextBreaksSingleCall(t *testing.T) {
	t.Parallel()

	service := NewMockAbsence(0)
	service.FailNext()

	_, err := service.Policy(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.Error(t, err)

	notes, err := service.Policy(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.NotEmpty(t, notes)
}

func TestMockRespectsDelay(t *testing.T) {
	t.Parallel()

	service := NewMockAbsence(20 * time.Millisecond)

	start := time.Now()
	_, err := service.Create(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
}
