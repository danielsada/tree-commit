# Example Monorepo

This is an example monorepo containing multiple projects and libraries.

## Structure

- `python_lib/` - Python mathematical utilities library
- `go_lib/` - Go utilities and configuration management
- `frontend/` - React TypeScript frontend application
- `scripts/` - Build and deployment scripts
- `docs/` - Documentation

## Getting Started

### Python Library

```bash
cd python_lib
pip install -r requirements.txt
python -m pytest tests/
```

### Go Library

```bash
cd go_lib
go mod tidy
go test ./...
go run cmd/main.go
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```
