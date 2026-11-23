// Command calc evaluates simple integer expressions: calc 6 '*' 7.
package main

import (
	"fmt"
	"os"
	"strconv"

	"example.com/monorepo/go_lib/pkg/mathutil"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: calc <a> <op> <b>")
		os.Exit(2)
	}
	a, errA := strconv.Atoi(os.Args[1])
	b, errB := strconv.Atoi(os.Args[3])
	if errA != nil || errB != nil {
		fmt.Fprintln(os.Stderr, "operands must be integers")
		os.Exit(2)
	}
	result, err := mathutil.Apply(os.Args[2], a, b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(result)
}
