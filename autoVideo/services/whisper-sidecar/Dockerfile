FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

ENV WHISPER_MODEL_SIZE=large-v3
ENV WHISPER_COMPUTE_TYPE=int8
ENV WHISPER_DEVICE=cpu
ENV PORT=8010

EXPOSE 8010

CMD ["python", "main.py"]
