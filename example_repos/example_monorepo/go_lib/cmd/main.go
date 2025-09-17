package main

import (
	"fmt"
	"log"

	"github.com/example/go_lib/pkg/mathutil"
)

func main() {
	fmt.Println("Go Library Example")

	result := mathutil.Add(5, 3)
	fmt.Printf("5 + 3 = %d\n", result)

	factorial := mathutil.Factorial(5)
	fmt.Printf("5! = %d\n", factorial)

	if err := mathutil.Validate(10); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("All operations completed successfully!")
}