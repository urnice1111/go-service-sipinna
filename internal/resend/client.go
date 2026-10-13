// Package resend envía correos electrónicos a través de la API de Resend.
package resend

import (
	"context"
	"errors"
	"fmt"
	"go-service-sipinna/internal/config"
	"html"
	"strings"

	"github.com/resend/resend-go/v4"
)

// defaultFrom es el remitente cuando SIPINNA_RESEND_FROM no está definido.
const defaultFrom = "SIPINNA <no-reply@sipinna.com>"

// SendPasswordResetCode envía a recipientEmail el código de recuperación de
// contraseña. name puede venir vacío; minutes es la vigencia que se muestra en el
// correo.
func SendPasswordResetCode(ctx context.Context, cfg *config.Config, recipientEmail, name, code string, minutes int) error {
	greeting := "Hola"
	if name = strings.TrimSpace(name); name != "" {
		greeting += " " + name
	}

	body := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;color:#1f2937">
  <p>%s,</p>
  <p>Recibimos una solicitud para restablecer la contraseña de tu cuenta de SIPINNA. Tu código es:</p>
  <p style="font-size:28px;font-weight:bold;letter-spacing:6px">%s</p>
  <p>El código vence en %d minutos. Si no solicitaste el cambio, ignora este correo; tu contraseña no se modificará.</p>
</div>`, html.EscapeString(greeting), code, minutes)

	text := fmt.Sprintf("%s,\n\nTu código para restablecer la contraseña de SIPINNA es: %s\n\nVence en %d minutos. Si no lo solicitaste, ignora este correo.\n",
		greeting, code, minutes)

	return send(ctx, cfg, recipientEmail, "Tu código de recuperación de SIPINNA", body, text)
}

// SendOneNotification avisa a un alimentador que se registró un reporte nuevo.
func SendOneNotification(ctx context.Context, cfg *config.Config, name, email string) error {
	greeting := "Hola"
	if name = strings.TrimSpace(name); name != "" {
		greeting += " " + name
	}

	body := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;color:#1f2937">
  <p>%s,</p>
  <p>Se registró un reporte nuevo en tu zona de SIPINNA. Entra a la plataforma para revisarlo.</p>
</div>`, html.EscapeString(greeting))

	text := fmt.Sprintf("%s,\n\nSe registró un reporte nuevo en tu zona de SIPINNA. Entra a la plataforma para revisarlo.\n", greeting)

	return send(ctx, cfg, email, "Nuevo reporte en tu zona de SIPINNA", body, text)
}

// send envía un correo con la API key y el remitente de cfg.
func send(ctx context.Context, cfg *config.Config, recipientEmail, subject, htmlBody, textBody string) error {
	if strings.TrimSpace(cfg.SipinnaResendAPIKey) == "" {
		return errors.New("SIPINNA_RESEND_API_KEY is not configured")
	}

	from := strings.TrimSpace(cfg.SipinnaResendFrom)
	if from == "" {
		from = defaultFrom
	}

	client := resend.NewClient(cfg.SipinnaResendAPIKey)

	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{recipientEmail},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
	}

	if _, err := client.Emails.SendWithContext(ctx, params); err != nil {
		return fmt.Errorf("send email with resend: %w", err)
	}

	return nil
}
