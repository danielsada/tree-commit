def add(a, b):
    """Add two numbers."""
    return a + b


def multiply(a, b):
    """Multiply two numbers."""
    return a * b


class Calculator:
    """A simple calculator class."""

    def __init__(self):
        self.history = []

    def calculate(self, operation, a, b):
        """Perform calculation and store in history."""
        if operation == "add":
            result = add(a, b)
        elif operation == "multiply":
            result = multiply(a, b)
        else:
            raise ValueError(f"Unknown operation: {operation}")

        self.history.append((operation, a, b, result))
        return result