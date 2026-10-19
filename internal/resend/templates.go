package resend

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"math"
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
	Notification, Urgent                              bool
	Message                                           string
	Veracity                                          *veracityView
}

// veracityView es la probabilidad de veracidad ya formateada para el correo.
type veracityView struct {
	Ready                bool   // false si el análisis aún no termina
	TruePct, FalsePct    int    // porcentajes que suman 100
	Level, Color, Detail string // nivel de confianza en texto, color del nivel y explicación
}

// Text es la versión en texto plano del análisis.
func (v *veracityView) Text() string {
	if !v.Ready {
		return "Análisis de veracidad: " + v.Detail
	}
	return fmt.Sprintf("Análisis de veracidad: %d %% probable que sea verídico, %d %% probable que sea falso (%s). %s",
		v.TruePct, v.FalsePct, v.Level, v.Detail)
}

// veracityContent convierte el puntaje de sospecha en porcentajes y un nivel.
func veracityContent(v *Veracity) *veracityView {
	if v == nil {
		return nil
	}
	if v.Suspicion == nil {
		return &veracityView{Detail: "El análisis automático todavía está en proceso. Consulta el nivel de sospecha en la plataforma."}
	}

	falsePct := int(math.Round(math.Min(math.Max(*v.Suspicion, 0), 1) * 100))
	view := &veracityView{Ready: true, TruePct: 100 - falsePct, FalsePct: falsePct,
		Detail: "Es una estimación automática: no descarta el reporte, atiéndelo según el protocolo."}
	switch {
	case view.TruePct >= 70:
		view.Level, view.Color = "probablemente verídico", "#1e7a46"
	case view.TruePct >= 40:
		view.Level, view.Color = "veracidad incierta", "#9a6200"
	default:
		view.Level, view.Color = "posiblemente falso", "#b42332"
	}
	return view
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

func renderReportNotificationEmail(greeting, message, link string, urgent bool, veracity *veracityView) (string, error) {
	title := "Nuevo reporte en tu zona"
	if urgent {
		title = "Reporte con peligro inmediato"
	}
	return renderEmail(emailContent{
		Title: title, Preheader: message, Greeting: greeting, Message: message,
		ActionURL: link, Notification: true, Urgent: urgent, Veracity: veracity,
	})
}

func renderEmail(content emailContent) (string, error) {
	var body bytes.Buffer
	if err := emailTemplate.Execute(&body, content); err != nil {
		return "", fmt.Errorf("render SIPINNA email: %w", err)
	}
	return body.String(), nil
}
