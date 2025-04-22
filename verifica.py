import jwt

token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6ImtpbGxlciIsImV4cCI6MTc0NTI0NjExOSwib3AiOnRydWV9.J0GbodKGHOV3u_j3uia4Y4vcQ6lofWHwstYXGPzkEEA"
decoded = jwt.decode(token, options={"verify_signature": False})
print(decoded)
