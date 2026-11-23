package mathutil

import (
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	tests := []struct {
		op   string
		a, b int
		want int
	}{
		{"+", 2, 3, 5},
		{"-", 2, 3, -1},
		{"*", 6, 7, 42},
		{"/", 7, 2, 3},
	}
	for _, tt := range tests {
		got, err := Apply(tt.op, tt.a, tt.b)
		if err != nil || got != tt.want {
			t.Errorf("Apply(%q, %d, %d) = %d, %v; want %d", tt.op, tt.a, tt.b, got, err, tt.want)
		}
	}
}

func TestApplyErrors(t *testing.T) {
	if _, err := Apply("/", 1, 0); !errors.Is(err, ErrDivideByZero) {
		t.Errorf("expected ErrDivideByZero, got %v", err)
	}
	if _, err := Apply("^", 1, 2); err == nil {
		t.Error("expected an error for an unknown operator")
	}
}

func TestFactorial(t *testing.T) {
	if got, err := Factorial(5); err != nil || got != 120 {
		t.Errorf("Factorial(5) = %d, %v", got, err)
	}
	if _, err := Factorial(-1); err == nil {
		t.Error("expected an error for a negative number")
	}
}
