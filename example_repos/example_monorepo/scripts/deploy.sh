#!/bin/bash

set -e

ENVIRONMENT=${1:-staging}

echo "Deploying to $ENVIRONMENT..."

# Run build first
./scripts/build.sh

case $ENVIRONMENT in
    "staging")
        echo "Deploying to staging environment..."
        docker-compose -f docker-compose.staging.yml down || true
        docker-compose -f docker-compose.staging.yml build
        docker-compose -f docker-compose.staging.yml up -d
        ;;
    "production")
        echo "Deploying to production environment..."
        read -p "Are you sure you want to deploy to production? (y/N): " confirm
        if [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]]; then
            docker-compose -f docker-compose.prod.yml down || true
            docker-compose -f docker-compose.prod.yml build
            docker-compose -f docker-compose.prod.yml up -d
        else
            echo "Deployment cancelled."
            exit 1
        fi
        ;;
    *)
        echo "Unknown environment: $ENVIRONMENT"
        echo "Usage: $0 [staging|production]"
        exit 1
        ;;
esac

echo "Deployment to $ENVIRONMENT completed!"