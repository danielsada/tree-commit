"""Formatting helpers."""


def format_result(value: float, precision: int = 2) -> str:
    """Format a numeric result, dropping the decimals of whole numbers."""
    if value == int(value):
        return str(int(value))
    return f"{value:.{precision}f}"
