import jwt
import datetime

secret = "autovideo-access-secret-change-in-prod"
payload = {
    "sub": "admin",
    "exp": datetime.datetime.utcnow() + datetime.timedelta(hours=1)
}
token = jwt.encode(payload, secret, algorithm="HS256")
print(token)
