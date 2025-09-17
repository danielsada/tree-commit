#!/usr/bin/env python3

import os
import subprocess
import sys


def run_command(cmd, cwd=None):
    """Run a shell command and return the result."""
    print(f"Running: {cmd}")
    result = subprocess.run(cmd, shell=True, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"Error: {result.stderr}")
        return False
    print(result.stdout)
    return True


def setup_python_lib():
    """Set up Python library dependencies."""
    print("Setting up Python library...")
    os.chdir("python_lib")

    # Create virtual environment if it doesn't exist
    if not os.path.exists("venv"):
        run_command("python -m venv venv")

    # Install dependencies
    run_command("pip install pytest black flake8")

    os.chdir("..")


def setup_go_lib():
    """Set up Go library dependencies."""
    print("Setting up Go library...")
    os.chdir("go_lib")

    # Initialize go module if needed
    if not os.path.exists("go.mod"):
        run_command("go mod init github.com/example/go_lib")

    run_command("go mod tidy")

    os.chdir("..")


def setup_frontend():
    """Set up frontend dependencies."""
    print("Setting up frontend...")
    os.chdir("frontend")

    # Initialize package.json if needed
    if not os.path.exists("package.json"):
        run_command("npm init -y")
        run_command("npm install react react-dom typescript @types/react @types/react-dom")

    os.chdir("..")


def main():
    """Main setup function."""
    print("Setting up monorepo development environment...")

    setup_python_lib()
    setup_go_lib()
    setup_frontend()

    print("Setup completed successfully!")


if __name__ == "__main__":
    main()