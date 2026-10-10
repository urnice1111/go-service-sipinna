// Package handlers contiene los handlers HTTP (Gin) de la API de SIPINNA.
//
// Cada constructor recibe sus dependencias (pool de PostgreSQL, configuración,
// cliente de S3) y regresa un [gin.HandlerFunc]. Los handlers de rutas protegidas
// asumen que antes corrió el middleware de autenticación, que deja en el contexto
// las llaves "user_id", "user_type" e "is_admin".
//
// Tipos de usuario:
//   - Ciudadano ("citizen"): crea reportes y consulta solo los suyos.
//   - Administrador ("administrador"): ve y gestiona reportes de todas las zonas y
//     administra las cuentas del personal.
//   - Alimentador ("alimentador"): ve y gestiona solo los reportes de su zona.
//
// El personal solo tiene acceso si su cuenta está "activada"; eso se verifica contra
// la base de datos en cada petición, no solo con el JWT.
package handlers
