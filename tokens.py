import jwt
import datetime

SECRET_KEY = "master333"  # debe coincidir con el settings.toml

def generar_token(username, es_op=False):
    ahora = datetime.datetime.utcnow()

    payload = {
        "sub": username,                     # estándar: subject
        "username": username,                # redundante pero útil
        "nick": "killer",                    # nombre visible
        "img": "/static/photos/killer.jpg",  # foto de perfil
        "url": "/u/killer",                  # URL de perfil
        "gender": "m",                       # género (m, f, o)
        "emoji": "🤖",                       # ícono de emoji
        "rules": ["redcam", "noimage"],      # reglas opcionales
        "iss": "my own app",                 # emisor
        "iat": ahora,
        "nbf": ahora,
        "exp": ahora + datetime.timedelta(hours=12)
    }

    if es_op:
        payload["op"] = True  # operador/moderador

    token = jwt.encode(payload, SECRET_KEY, algorithm="HS256")
    return token

if __name__ == "__main__":
    token = generar_token("killer", es_op=True)
    print("JWT Token:", token)
    print(f"Úsalo en la URL: http://localhost:9000/?jwt={token}")
