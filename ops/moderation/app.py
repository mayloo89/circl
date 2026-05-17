"""
NSFW image-classification sidecar for the Circl moderation pipeline.

Why a sidecar?
    NudeNet ships as a Python library on top of ONNX runtime. Running it as
    a separate service keeps the Go backend free of CGo dependencies and the
    ~12 MB ONNX model out of the Go binary, and lets the moderation model be
    swapped (or scaled independently) without touching the rest of the app.

Contract:
    POST /classify   (multipart/form-data, field "file"|"image")
        → 200 { "nsfw_score": float [0,1], "categories": [str], "detections": [...] }
        → 400 { "error": "..." } on a bad request
    GET  /healthz
        → 200 "ok" once the model is loaded

The Go side hits /classify; /healthz is for the docker-compose healthcheck
so the backend container only marks the moderation chain "alive" when the
model is actually loaded (NudeNet does a one-off ONNX init on first request
that can take 5-10 s on a cold container).
"""

from __future__ import annotations

import io
import logging
import os
import tempfile
from typing import Final

from fastapi import FastAPI, File, HTTPException, UploadFile
from nudenet import NudeDetector
from PIL import Image

# NudeNet labels that we treat as "explicit" for scoring purposes. The model
# also emits "covered" variants (e.g. FEMALE_BREAST_COVERED) — those are NOT
# in this set on purpose; soft/SFW imagery should not be flagged.
EXPOSED_LABELS: Final[set[str]] = {
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
    "ANUS_EXPOSED",
    "FEMALE_BREAST_EXPOSED",
    "BUTTOCKS_EXPOSED",
}

# Max image dimension we accept — anything bigger gets downscaled before the
# detector runs. Keeps inference time bounded and the model well-fed (NudeNet
# was trained around 320 px input).
MAX_PX: Final[int] = 1024

# Max bytes accepted per request. The Go side already enforces upload limits;
# this is defense-in-depth.
MAX_BYTES: Final[int] = 25 * 1024 * 1024

logger = logging.getLogger("moderation")
logging.basicConfig(level=os.environ.get("LOG_LEVEL", "INFO"))

app = FastAPI(title="circl-moderation", version="1.0")
detector = NudeDetector()


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/classify")
async def classify(file: UploadFile = File(...)) -> dict[str, object]:
    raw = await file.read()
    if len(raw) == 0:
        raise HTTPException(status_code=400, detail="empty file")
    if len(raw) > MAX_BYTES:
        raise HTTPException(status_code=400, detail="file too large")

    # Re-encode through PIL so we (a) reject corrupt or non-image payloads
    # before they hit the detector, (b) cap dimensions, and (c) hand the
    # detector a clean JPEG path which is its fastest input.
    try:
        img = Image.open(io.BytesIO(raw)).convert("RGB")
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=400, detail=f"invalid image: {exc}") from exc

    if max(img.size) > MAX_PX:
        img.thumbnail((MAX_PX, MAX_PX))

    with tempfile.NamedTemporaryFile(suffix=".jpg", delete=True) as tmp:
        img.save(tmp.name, "JPEG", quality=92)
        detections = detector.detect(tmp.name) or []

    # Score = max confidence across any exposed-class detection. This is the
    # signal the Go side compares against its threshold; categories are
    # returned for the audit trail.
    exposed = [d for d in detections if d.get("class") in EXPOSED_LABELS]
    score = max((float(d.get("score", 0.0)) for d in exposed), default=0.0)
    categories = sorted({d["class"] for d in exposed})

    return {
        "nsfw_score": score,
        "categories": categories,
        "detections": detections,
    }
