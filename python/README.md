# ipc

Python client helpers for stdio-based IPC.

## Layout
- `ipc/`: package code
- `pyproject.toml`: build metadata

## Local development
```sh
uv venv
uv sync
```

## Usage
```python
from ipc import Endpoint, Router, new_request
```
