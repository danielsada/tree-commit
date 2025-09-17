import pytest
from src.calculator import Calculator, add, multiply


def test_add():
    """Test the add function."""
    assert add(2, 3) == 5
    assert add(-1, 1) == 0
    assert add(0, 0) == 0


def test_multiply():
    """Test the multiply function."""
    assert multiply(2, 3) == 6
    assert multiply(-2, 3) == -6
    assert multiply(0, 5) == 0


class TestCalculator:
    """Test cases for Calculator class."""

    def test_calculator_add(self):
        """Test calculator add operation."""
        calc = Calculator()
        result = calc.calculate("add", 5, 3)
        assert result == 8
        assert len(calc.history) == 1

    def test_calculator_multiply(self):
        """Test calculator multiply operation."""
        calc = Calculator()
        result = calc.calculate("multiply", 4, 3)
        assert result == 12
        assert len(calc.history) == 1

    def test_unknown_operation(self):
        """Test calculator with unknown operation."""
        calc = Calculator()
        with pytest.raises(ValueError):
            calc.calculate("divide", 10, 2)