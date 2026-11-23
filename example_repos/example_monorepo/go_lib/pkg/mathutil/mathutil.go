// Package mathutil implements integer arithmetic with error checking.
package mathutil

import (
	"errors"
	"fmt"
)

// ErrDivideByZero is returned when dividing by zero.
var ErrDivideByZero = errors.New("divide by zero")

// Apply evaluates a op b.
func Apply(op string, a, b int) (int, error) {
	switch op {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return 0, ErrDivideByZero
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("unknown operator %q", op)
	}
}

// Factorial returns n! for n >= 0.
func Factorial(n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("factorial of negative number %d", n)
	}
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result, nil
}
