# Deployment Guide

## Prerequisites

- Docker installed
- Node.js 16+ for frontend
- Python 3.8+ for Python library
- Go 1.19+ for Go library

## Build Process

### Python Library

```bash
cd python_lib
python setup.py sdist bdist_wheel
```

### Go Library

```bash
cd go_lib
go build -o bin/app cmd/main.go
```

### Frontend

```bash
cd frontend
npm run build
```

## Docker Deployment

Each component has its own Dockerfile:

```bash
# Build all containers
docker-compose build

# Deploy to staging
docker-compose -f docker-compose.staging.yml up -d

# Deploy to production
docker-compose -f docker-compose.prod.yml up -d
```

## Environment Variables

- `ENVIRONMENT` - development, staging, production
- `DEBUG` - true/false for debug mode
- `API_URL` - Backend API endpoint
- `DATABASE_URL` - Database connection string