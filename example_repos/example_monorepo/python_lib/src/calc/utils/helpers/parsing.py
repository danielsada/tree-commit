"""Parse expressions such as ``"3 * 4"``."""

from calc.core.engine import Operation

SYMBOLS = {
    "+": Operation.ADD,
    "-": Operation.SUBTRACT,
    "*": Operation.MULTIPLY,
    "/": Operation.DIVIDE,
}


def parse_expression(text: str) -> tuple[Operation, float, float]:
    """Split ``"<left> <symbol> <right>"`` into its parts."""
    parts = text.split()
    if len(parts) != 3:
        raise ValueError(f"expected '<left> <op> <right>', got {text!r}")
    left, symbol, right = parts
    if symbol not in SYMBOLS:
        raise ValueError(f"unknown operator {symbol!r}")
    return SYMBOLS[symbol], float(left), float(right)
