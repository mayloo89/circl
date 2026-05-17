# NudeNet moderation sidecar. Single-purpose container: FastAPI server in
# front of the NudeNet detector. Image is ~400 MB once NudeNet pulls
# onnxruntime — large by container standards but fine for a sidecar that
# stays running.
FROM python:3.11-slim AS base

# System deps NudeNet/Pillow need at runtime.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libglib2.0-0 libsm6 libxext6 libxrender1 libjpeg-dev zlib1g-dev \
        curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python deps first so the layer caches across app changes.
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Warm the NudeNet model cache at build time so the first request after
# `docker compose up` doesn't pay the download cost. The library lazily
# downloads the ONNX model on first instantiation.
RUN python -c "from nudenet import NudeDetector; NudeDetector()"

COPY app.py .

ENV LOG_LEVEL=INFO

EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
    CMD curl --fail http://localhost:8000/healthz || exit 1

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
