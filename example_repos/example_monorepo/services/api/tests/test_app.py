import json

from api.app import application


def call(path: str, query: str = "") -> tuple[str, dict]:
    statuses = []
    body = application(
        {"PATH_INFO": path, "QUERY_STRING": query},
        lambda status, headers: statuses.append(status),
    )
    return statuses[0], json.loads(b"".join(body))


def test_add():
    assert call("/add", "a=1&b=2") == ("200 OK", {"result": 3.0})


def test_unknown_operation():
    status, _ = call("/power", "a=1&b=2")
    assert status == "400 Bad Request"


def test_missing_argument():
    status, _ = call("/add", "a=1")
    assert status == "400 Bad Request"
