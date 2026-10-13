package resend

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
)

//go:embed email.html
var emailHTML string

// html/template elimina comentarios HTML. Estos fragmentos constantes conservan
// el contenedor condicional de Outlook sin marcar datos del usuario como HTML.
var emailTemplate = template.Must(template.New("sipinna-email").Funcs(template.FuncMap{
	"outlookStart": func() template.HTML {
		return `<!--[if mso]><table role="presentation" width="600"><tr><td><![endif]-->`
	},
	"outlookEnd": func() template.HTML {
		return `<!--[if mso]></td></tr></table><![endif]-->`
	},
}).Parse(emailHTML))

type emailContent struct {
	Title, Preheader, Greeting, Role, ActionURL, Code string
	Validity                                          int
	Invitation                                        bool
}

func renderPasswordResetEmail(greeting, code string, minutes int) (string, error) {
	return renderEmail(emailContent{
		Title: "Recupera el acceso a tu cuenta", Preheader: "Tu código para restablecer la contraseña de SIPINNA.",
		Greeting: greeting, Code: code, Validity: minutes,
	})
}

func renderStaffInvitationEmail(greeting, role, link string, hours int) (string, error) {
	return renderEmail(emailContent{
		Title: "Te damos la bienvenida a SIPINNA", Preheader: "Acepta tu invitación y activa tu cuenta de personal.",
		Greeting: greeting, Role: role, ActionURL: link, Validity: hours, Invitation: true,
	})
}

func renderEmail(content emailContent) (string, error) {
	var body bytes.Buffer
	if err := emailTemplate.Execute(&body, content); err != nil {
		return "", fmt.Errorf("render SIPINNA email: %w", err)
	}
	return body.String(), nil
}
