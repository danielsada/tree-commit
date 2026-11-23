"""A calculator that keeps a history of the operations it performed."""

from __future__ import annotations

import operator
from dataclasses import dataclass, field
from enum import Enum
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable


class Operation(Enum):
    """Operations supported by the calculator."""

    ADD = "add"
    SUBTRACT = "subtract"
    MULTIPLY = "multiply"
    DIVIDE = "divide"


_FUNCTIONS: dict[Operation, Callable[[float, float], float]] = {
    Operation.ADD: operator.add,
    Operation.SUBTRACT: operator.sub,
    Operation.MULTIPLY: operator.mul,
    Operation.DIVIDE: operator.truediv,
}


@dataclass(frozen=True)
class Entry:
    """A calculation recorded in the history."""

    operation: Operation
    left: float
    right: float
    result: float


@dataclass
class Calculator:
    """Performs calculations and remembers them."""

    history: list[Entry] = field(default_factory=list)

    def calculate(self, operation: Operation | str, left: float, right: float) -> float:
        """Apply ``operation`` to the operands and record the result.

        Raises:
            ValueError: if the operation is unknown or divides by zero.

        """
        op = Operation(operation)
        if op is Operation.DIVIDE and right == 0:
            msg = "cannot divide by zero"
            raise ValueError(msg)
        result = _FUNCTIONS[op](left, right)
        self.history.append(Entry(op, left, right, result))
        return result

    def undo(self) -> Entry | None:
        """Forget the last calculation and return it."""
        return self.history.pop() if self.history else None
