# Development

All Python projects are managed with [uv](https://docs.astral.sh/uv/).
You do not need to create virtual environments by hand.

## Python

```sh
cd python_lib
uv sync
uv run pytest
```

Lint and format with ruff:

```sh
uvx ruff check --fix
uvx ruff format
```

## Go

```sh
cd go_lib
go test ./...
```

## Frontend

```sh
cd frontend
npm install
npm run dev
```
