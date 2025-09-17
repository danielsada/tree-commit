import json
from typing import List, Dict, Any


def load_config(file_path: str) -> Dict[str, Any]:
    """Load configuration from JSON file."""
    with open(file_path, 'r') as f:
        return json.load(f)


def validate_numbers(numbers: List[float]) -> bool:
    """Validate that all items in list are numbers."""
    return all(isinstance(num, (int, float)) for num in numbers)


def format_result(value: float, precision: int = 2) -> str:
    """Format a numeric result to specified precision."""
    return f"{value:.{precision}f}"