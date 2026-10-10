// Package repository encapsula el acceso a PostgreSQL (pgx) y a S3.
//
// Cada función abre su propio contexto con un timeout corto (normalmente 5 s).
// Cuando un registro no existe o queda fuera del alcance del usuario, las funciones
// regresan [pgx.ErrNoRows] para que el handler responda 404.
//
// Varias consultas aceptan un scopeZoneID: si es nil no se filtra por zona
// (administrador) y si no es nil el resultado se limita a esa zona (alimentador).
package repository
