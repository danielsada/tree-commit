package mathutil

import "errors"

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	return a * b
}

// Factorial calculates the factorial of n
func Factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * Factorial(n-1)
}

// Validate checks if a number is positive
func Validate(n int) error {
	if n < 0 {
		return errors.New("number must be positive")
	}
	return nil
}