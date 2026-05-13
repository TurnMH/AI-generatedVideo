import requests
import jwt
import time

secret = "autovideo-access-secret-change-in-prod"
url = "http://127.0.0.1:8004/api/v1/chat"
models = ["gemini-3.1-pro-preview", "gemini-3-pro-preview", "gemini-2.5-pro", "gemini-2.5-flash"]

def get_token():
    payload = {
        "user_id": 1,
        "exp": int(time.time()) + 3600
    }
    return jwt.encode(payload, secret, algorithm="HS256")

token = get_token()
headers = {
    "Authorization": f"Bearer {token}",
    "Content-Type": "application/json"
}

for model in models:
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": "hi"}]
    }
    start = time.time()
    try:
        resp = requests.post(url, json=payload, headers=headers, timeout=60)
        elapsed = time.time() - start
        print(f"Model: {model}")
        print(f"Status: {resp.status_code}")
        print(f"Elapsed: {elapsed:.2f}s")
        print(f"Excerpt: {resp.text[:100]}...")
    except Exception as e:
        print(f"Model: {model} failed: {e}")
    print("-" * 20)
