const crypto = require('crypto');

function base64url(buf) {
  return buf.toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

const header = {
  "alg": "HS256",
  "typ": "JWT"
};

const payload = {
  "sub": "1",
  "project_id": 1,
  "iat": Math.floor(Date.now() / 1000) - 10000,
  "exp": Math.floor(Date.now() / 1000) + 3600
};

const encodedHeader = base64url(Buffer.from(JSON.stringify(header)));
const encodedPayload = base64url(Buffer.from(JSON.stringify(payload)));

const signature = crypto.createHmac('sha256', 'autovideo-access-secret-change-in-prod')
  .update(encodedHeader + "." + encodedPayload)
  .digest();

const encodedSignature = base64url(signature);

console.log(encodedHeader + "." + encodedPayload + "." + encodedSignature);
