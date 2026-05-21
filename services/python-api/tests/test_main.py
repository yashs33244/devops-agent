import pytest
from fastapi.testclient import TestClient
from main import app

client = TestClient(app)

def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"

def test_list_items():
    r = client.get("/api/items")
    assert r.status_code == 200
    assert len(r.json()["items"]) == 2

def test_get_item():
    r = client.get("/api/items/1")
    assert r.status_code == 200
    assert r.json()["id"] == 1

def test_item_not_found():
    r = client.get("/api/items/999")
    assert r.status_code == 404

def test_metrics_endpoint():
    r = client.get("/metrics")
    assert r.status_code == 200
    assert "http_requests_total" in r.text
