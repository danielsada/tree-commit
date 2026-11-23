"""A dependency-free WSGI application: ``GET /add?a=1&b=2``."""

import json
from collections.abc import Callable, Iterable
from urllib.parse import parse_qs

OPERATIONS: dict[str, Callable[[float, float], float]] = {
    "add": lambda a, b: a + b,
    "subtract": lambda a, b: a - b,
    "multiply": lambda a, b: a * b,
}


def application(environ: dict, start_response: Callable) -> Iterable[bytes]:
    operation = environ.get("PATH_INFO", "").strip("/")
    query = parse_qs(environ.get("QUERY_STRING", ""))
    try:
        func = OPERATIONS[operation]
        result = func(float(query["a"][0]), float(query["b"][0]))
    except (KeyError, ValueError):
        return _respond(start_response, "400 Bad Request", {"error": "bad request"})
    return _respond(start_response, "200 OK", {"result": result})


def _respond(start_response: Callable, status: str, body: dict) -> list[bytes]:
    start_response(status, [("Content-Type", "application/json")])
    return [json.dumps(body).encode()]
