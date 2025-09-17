# API Documentation

## Python Library API

### Calculator Class

#### Methods

- `add(a, b)` - Returns the sum of two numbers
- `multiply(a, b)` - Returns the product of two numbers
- `calculate(operation, a, b)` - Performs calculation and stores in history

### Utils Module

- `load_config(file_path)` - Load configuration from JSON file
- `validate_numbers(numbers)` - Validate list contains only numbers
- `format_result(value, precision=2)` - Format numeric result

## Go Library API

### MathUtil Package

```go
func Add(a, b int) int
func Multiply(a, b int) int
func Factorial(n int) int
func Validate(n int) error
```

### Config Package

```go
type Config struct {
    AppName string `json:"app_name"`
    Version string `json:"version"`
    Debug   bool   `json:"debug"`
}

func Load(filename string) (*Config, error)
func (c *Config) Save(filename string) error
```