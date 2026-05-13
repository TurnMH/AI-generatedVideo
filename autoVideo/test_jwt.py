import jwt
import time
import requests
import json

SECRET = "autovideo-access-secret-change-in-prod"
ALGORITHM = "HS256"

def create_token(claims):
    return jwt.encode(claims, SECRET, algorithm=ALGORITHM)

def test_request(token, label):
    print(f"\n--- Testing {label} ---")
    headers = {"Authorization": f"Bearer {token}"}
    
    # Test GET
    try:
        r_get = requests.get("http://127.0.0.1:8002/api/v1/projects/4", headers=headers)
        print(f"GET /projects/4: {r_get.status_code}")
        print(f"Response: {r_get.text}")
    except Exception as e:
        print(f"GET Error: {e}")

    # Test POST (only for the first/main token variant)
    if label == "Main Token":
        try:
            body = {
                "episode_id": 76,
                "model_name": "gpt-image-1",
                "model_names": ["gpt-image-1", "doubao-image", "wan2.5-i2i-preview"]
            }
            r_post = requests.post("http://127.0.0.1:8002/api/v1/projects/4/storyboards/retry-failed", 
                                   headers=headers, json=body)
            print(f"POST /retry-failed: {r_post.status_code}")
            print(f"Response: {r_post.text}")
        except Exception as e:
            print(f"POST Error: {e}")

now = int(time.time())

# Variant 1: Original requirement
claims1 = {
  "user_id": 1,
  "project_id": 0,
  "role": "service",
  "token_type": "access",
  "iat": now,
  "exp": now + 300
}
test_request(create_token(claims1), "Main Token")

# Variant 2: user_id as string
claims2 = claims1.copy()
claims2["user_id"] = "1"
test_request(create_token(claims2), "user_id as string")

# Variant 3: No project_id
claims3 = claims1.copy()
del claims3["project_id"]
test_request(create_token(claims3), "No project_id")

# Variant 4: No token_type
claims4 = claims1.copy()
del claims4["token_type"]
test_request(create_token(claims4), "No token_type")

