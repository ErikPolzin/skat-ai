package agent

import (
	"errors"
	"testing"
)

func TestErrorActionReturnsErrorMessage(t *testing.T) {
	want := errors.New("agent unavailable")

	response, err := ErrorAction(want)()

	if !errors.Is(err, want) {
		t.Fatalf("expected original error, got %v", err)
	}
	if response != want.Error() {
		t.Fatalf("expected response %q, got %q", want.Error(), response)
	}
}
