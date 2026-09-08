# go-service-sipinna

Backend del sistema SIPINNA. API en Go (Gin) con PostgreSQL.

---

## Levantar el proyecto desde cero

### 1. Base de datos (PostgreSQL en Docker)

```bash
docker run --name sipinna-db \
  -e POSTGRES_PASSWORD=password123 \
  -e POSTGRES_DB=sipinna \
  -p 5432:5432 \
  -d postgres
```

Si el contenedor ya existe y solo está apagado:

```bash
docker start sipinna-db
```

### 2. Crear las tablas (migraciones)

```bash
docker exec -i sipinna-db psql -U postgres -d sipinna < migrations/000001_create_whole_database.up.sql
```

### 3. Archivo `.env`

En la raíz del proyecto (no se sube a Git, cada quien crea el suyo):

```bash
cat > .env << 'EOF'
DATABASE_URL=postgres://postgres:password123@localhost:5432/sipinna
PORT=8080
JWT_SECRET=mi_secreto_jwt_seguro_123
EOF
```

### 4. Correr el servidor

```bash
go run cmd/api/main.go
```

Debe mostrar `Listening and serving HTTP on :8080`.

---

## Conectarse a la base de datos

```bash
docker exec -it sipinna-db psql -U postgres -d sipinna
```

**Importante:** `-U postgres` es obligatorio.

Dentro del contenedor tu usuario de Linux es `root`, pero PostgreSQL solo tiene el rol
`postgres`. Son dos sistemas de usuarios distintos. Si corres `psql` sin `-U postgres`,
intenta conectarse con el rol `root` y falla con:

```
FATAL: role "root" does not exist
```

### Comandos útiles dentro de psql

| Comando | Qué hace |
|---|---|
| `\dt` | Lista las tablas |
| `\d usuarios` | Muestra las columnas de la tabla `usuarios` |
| `\q` | Salir |

### Consultar sin entrar a psql

```bash
docker exec sipinna-db psql -U postgres -d sipinna -c "SELECT nombre, email, created_at FROM usuarios;"
```

---

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/` | Health check |
| `POST` | `/user` | Registrar ciudadano |

### `POST /user`

```json
{
  "nombre": "Juan Pérez",
  "edad": 25,
  "genero": "masculino",
  "email": "juan@ejemplo.com",
  "password": "mipass123"
}
```

Reglas:

- `nombre` y `password` son obligatorios
- Se requiere `email` **o** `telefono` (al menos uno)
- `telefono` en formato internacional: `+521234567890`
- `password` mínimo 6 caracteres

Respuesta exitosa: `201 Created` con el usuario creado.

---

## Conectar desde la app Android

El emulador no alcanza `localhost` de tu Mac directamente. Crea un túnel:

```bash
adb reverse tcp:8080 tcp:8080
```

Con eso la app usa `http://127.0.0.1:8080/` como base URL.

El túnel se borra al reiniciar el emulador, hay que volver a correrlo.

---

## Checklist diario

```bash
docker start sipinna-db                          # 1. base de datos
cd ~/go-service-sipinna && go run cmd/api/main.go # 2. servidor
adb reverse tcp:8080 tcp:8080                     # 3. túnel (solo para la app)
```
