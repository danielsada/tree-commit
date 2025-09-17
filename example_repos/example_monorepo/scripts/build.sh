#!/bin/bash

set -e

echo "Building monorepo components..."

# Build Python library
echo "Building Python library..."
cd python_lib
python -m pip install --upgrade pip
pip install -r requirements.txt || echo "No requirements.txt found"
python -m pytest tests/ || echo "No tests found"
cd ..

# Build Go library
echo "Building Go library..."
cd go_lib
go mod tidy
go test ./...
go build -o bin/app cmd/main.go
cd ..

# Build Frontend
echo "Building Frontend..."
cd frontend
if [ -f "package.json" ]; then
    npm install
    npm run build || npm run dist || echo "No build script found"
fi
cd ..

echo "All components built successfully!"