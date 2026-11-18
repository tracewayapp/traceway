# inventory-api

Stock levels for the warehouse team. FastAPI + uvicorn.

Local development uses a virtualenv in `.venv`:

```
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
.venv/bin/python -m unittest
PATH="$PWD/.venv/bin:$PATH" PORT=8000 ./start.sh
curl localhost:8000/api/items/1
```

Production builds the image from `Dockerfile` and runs `./start.sh`; the
environment comes from the deployment's env file.
