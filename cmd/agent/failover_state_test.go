package main

import "testing"

func TestFailoverState_GetNextServer_RotatesAfterRetryCount(t *testing.T) {
	f := &failoverState{
		servers:        []string{"main", "backup"},
		currentIdx:     0,
		attempts:       0,
		retryCount:     2,
		fullCyclePause: 0, // без sleep в тесте
	}

	// Контракт: каждый сервер пробуется retryCount раз, затем переключение.
	got := []string{
		f.getNextServer(),
		f.getNextServer(),
		f.getNextServer(),
		f.getNextServer(),
		f.getNextServer(),
		f.getNextServer(),
	}
	want := []string{"main", "main", "backup", "backup", "main", "main"}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d: expected %q, got %q (got=%v)", i, want[i], got[i], got)
		}
	}
}

func TestFailoverState_ResetAttempts_PreventsFalseFailover(t *testing.T) {
	f := &failoverState{
		servers:        []string{"main", "backup"},
		currentIdx:     0,
		attempts:       1,
		retryCount:     2,
		fullCyclePause: 0,
	}

	f.resetAttempts()
	if f.attempts != 0 {
		t.Fatalf("ожидали attempts=0 после resetAttempts, получили %d", f.attempts)
	}

	// Следующая попытка должна остаться на текущем сервере.
	if s := f.getNextServer(); s != "main" {
		t.Fatalf("ожидали, что после resetAttempts текущий сервер останется 'main', получили %q", s)
	}
}

