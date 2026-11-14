package integrations

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type AbsenceInput struct {
	DateFrom    string
	DateTo      string
	AbsenceType string
	Comment     string
}

type AbsenceResult struct {
	ExternalID string
	Status     string
}

type AbsenceService interface {
	Policy(ctx context.Context, input AbsenceInput) ([]string, error)
	Create(ctx context.Context, input AbsenceInput) (AbsenceResult, error)
}

type MockAbsence struct {
	mutex   sync.Mutex
	counter int
	delay   time.Duration
	failing bool
}

func NewMockAbsence(delay time.Duration) *MockAbsence {
	return &MockAbsence{delay: delay}
}

func (m *MockAbsence) Policy(_ context.Context, input AbsenceInput) ([]string, error) {
	if err := m.wait(); err != nil {
		return nil, err
	}

	notes := []string{
		"Отгул согласует руководитель команды",
	}

	switch input.AbsenceType {
	case "vacation":
		notes = append(notes, "Отпускные дни начисляются автоматически")
	case "sick_leave":
		notes = append(notes, "При болезни нужен больничный лист")
	default:
		notes = append(notes, "В день отгула отчётность не требуется")
	}

	return notes, nil
}

func (m *MockAbsence) Create(_ context.Context, input AbsenceInput) (AbsenceResult, error) {
	if err := m.wait(); err != nil {
		return AbsenceResult{}, err
	}

	m.mutex.Lock()
	m.counter++
	number := 1024 + m.counter
	m.mutex.Unlock()

	return AbsenceResult{
		ExternalID: fmt.Sprintf("ABS-%d", number),
		Status:     "pending",
	}, nil
}

func (m *MockAbsence) FailNext() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.failing = true
}

func (m *MockAbsence) wait() error {
	m.mutex.Lock()
	failing := m.failing
	m.failing = false
	delay := m.delay
	m.mutex.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if failing {
		return fmt.Errorf("integrations: mock absence service is unavailable")
	}

	return nil
}
