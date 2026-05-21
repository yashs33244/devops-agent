import time
import os
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import PlainTextResponse
from prometheus_client import Counter, Histogram, Gauge, generate_latest, CONTENT_TYPE_LATEST
import uvicorn

app = FastAPI(title="python-api", version="1.0.0")

# Prometheus metrics
REQUEST_COUNT = Counter("http_requests_total", "Total HTTP requests", ["method", "endpoint", "status"])
REQUEST_LATENCY = Histogram("http_request_duration_seconds", "HTTP request latency", ["endpoint"])
ACTIVE_REQUESTS = Gauge("http_active_requests", "Active HTTP requests")
start_time = time.time()

@app.middleware("http")
async def metrics_middleware(request: Request, call_next):
    ACTIVE_REQUESTS.inc()
    start = time.time()
    response = await call_next(request)
    duration = time.time() - start
    REQUEST_COUNT.labels(request.method, request.url.path, response.status_code).inc()
    REQUEST_LATENCY.labels(request.url.path).observe(duration)
    ACTIVE_REQUESTS.dec()
    return response

@app.get("/health")
async def health():
    return {"status": "ok", "service": "python-api", "uptime": time.time() - start_time}

@app.get("/readyz")
async def ready():
    return {"ready": True}

@app.get("/metrics", response_class=PlainTextResponse)
async def metrics():
    return PlainTextResponse(generate_latest(), media_type=CONTENT_TYPE_LATEST)

@app.get("/api/items")
async def list_items():
    return {"items": [{"id": 1, "name": "Widget"}, {"id": 2, "name": "Gadget"}]}

@app.get("/api/items/{item_id}")
async def get_item(item_id: int):
    if item_id not in (1, 2):
        raise HTTPException(status_code=404, detail="Item not found")
    return {"id": item_id, "name": "Widget" if item_id == 1 else "Gadget"}

@app.post("/api/items")
async def create_item(item: dict):
    return {"id": 3, "name": item.get("name", "New Item"), "created": True}

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=int(os.getenv("PORT", 8000)), reload=False)
