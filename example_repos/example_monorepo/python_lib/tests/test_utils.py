import pytest

from calc import Operation, format_result
from calc.utils.helpers.parsing import parse_expression


@pytest.mark.parametrize(
    ("value", "expected"),
    [(4.0, "4"), (2.5, "2.50"), (1 / 3, "0.33")],
)
def test_format_result(value, expected):
    assert format_result(value) == expected


def test_parse_expression():
    assert parse_expression("3 * 4") == (Operation.MULTIPLY, 3.0, 4.0)


@pytest.mark.parametrize("text", ["3 *", "3 ^ 4"])
def test_parse_expression_errors(text):
    with pytest.raises(ValueError):
        parse_expression(text)
