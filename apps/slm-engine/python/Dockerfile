FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY serve_privacy_filter.py .

ENV PORT=8086
ENV PRIVACY_FILTER_MODEL_PATH=openai/privacy-filter

# Create a non-root user
RUN useradd -m -u 1000 -U -s /bin/sh ocultar
RUN chown -R ocultar:ocultar /app
USER ocultar

CMD ["python", "serve_privacy_filter.py"]
