import pytest

from calc import Calculator, Operation


def test_calculate_records_history():
    calc = Calculator()
    assert calc.calculate("add", 2, 3) == 5
    assert calc.calculate(Operation.MULTIPLY, 4, 2.5) == 10
    assert [entry.operation for entry in calc.history] == [Operation.ADD, Operation.MULTIPLY]


def test_divide_by_zero():
    with pytest.raises(ValueError, match="divide by zero"):
        Calculator().calculate("divide", 1, 0)


def test_unknown_operation():
    with pytest.raises(ValueError):
        Calculator().calculate("power", 2, 3)


def test_undo():
    calc = Calculator()
    assert calc.undo() is None
    calc.calculate("subtract", 5, 3)
    assert calc.undo().result == 2
    assert calc.history == []
