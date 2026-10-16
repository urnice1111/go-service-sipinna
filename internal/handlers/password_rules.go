package handlers

import (
	"unicode"
	"unicode/utf8"
)

// Reglas de la contraseña (las mismas que revisa la app):
//   - De 10 a 128 caracteres
//   - Se permiten letras, números, símbolos y espacios
//   - Al menos una mayúscula
//   - Al menos un carácter especial (algo que no sea letra, número ni espacio)
const (
	passwordMinLength = 10
	passwordMaxLength = 128
	bcryptMaxBytes    = 72 // bcrypt no acepta contraseñas de más de 72 bytes
)

// validatePassword revisa que la contraseña cumpla las reglas. Si no las cumple
// regresa el mensaje de error para el usuario; si todo está bien regresa "".
func validatePassword(password string) string {
	length := utf8.RuneCountInString(password) // cuenta letras, no bytes (la ñ cuenta como 1)
	if length < passwordMinLength {
		return "La contraseña debe tener al menos 10 caracteres."
	}
	if length > passwordMaxLength || len(password) > bcryptMaxBytes {
		return "La contraseña es demasiado larga."
	}

	hasUpper := false
	hasSpecial := false
	for _, r := range password {
		if unicode.IsControl(r) {
			return "La contraseña no puede tener saltos de línea ni tabuladores."
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) {
			hasSpecial = true
		}
	}

	if !hasUpper {
		return "La contraseña debe tener al menos una letra mayúscula."
	}
	if !hasSpecial {
		return "La contraseña debe tener al menos un carácter especial (por ejemplo ! @ # $ %)."
	}
	return ""
}
