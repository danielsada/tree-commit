"""Print the next semantic version: ``uv run scripts/bump_version.py 1.2.3 minor``."""

import sys

PARTS = ("major", "minor", "patch")


def bump(version: str, part: str) -> str:
    numbers = [int(n) for n in version.split(".")]
    index = PARTS.index(part)
    numbers[index] += 1
    numbers[index + 1 :] = [0] * (len(numbers) - index - 1)
    return ".".join(str(n) for n in numbers)


if __name__ == "__main__":
    print(bump(sys.argv[1], sys.argv[2]))
