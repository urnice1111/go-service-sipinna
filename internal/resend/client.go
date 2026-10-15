// Package resend envía correos electrónicos a través de la API de Resend.
package resend

import (
	"context"
	"errors"
	"fmt"
	"go-service-sipinna/internal/config"
	"net/url"
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

	body, err := renderPasswordResetEmail(greeting, code, minutes)
	if err != nil {
		return err
	}

	text := fmt.Sprintf("%s,\n\nTu código para restablecer la contraseña de SIPINNA es: %s\n\nVence en %d minutos. Si no lo solicitaste, ignora este correo.\n",
		greeting, code, minutes)

	return send(ctx, cfg, recipientEmail, "Tu código de recuperación de SIPINNA", body, text)
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

// SendStaffInvitation envía a recipientEmail la invitación para activar su cuenta de personal.
func SendStaffInvitation(ctx context.Context, cfg *config.Config, recipientEmail, name, roleLabel, link string, hours int) error {
	greeting := "Hola"
	if name = strings.TrimSpace(name); name != "" {
		greeting += " " + name
	}

	body, err := renderStaffInvitationEmail(greeting, roleLabel, link, hours)
	if err != nil {
		return err
	}

	text := fmt.Sprintf("%s,\n\nTe invitaron a unirte a SIPINNA como %s. Para activar tu cuenta, abre este enlace y crea tu contraseña:\n\n%s\n\nVence en %d horas. Si no esperabas este correo, ignóralo.\n",
		greeting, roleLabel, link, hours)

	return send(ctx, cfg, recipientEmail, "Tu invitación a SIPINNA", body, text)
}

// SendOneNotification avisa a un alimentador de un reporte nuevo
func SendOneNotification(ctx context.Context, cfg *config.Config, name, email string, urgent bool) error {
	greeting := "Hola"
	if name = strings.TrimSpace(name); name != "" {
		greeting += " " + name
	}

	subject := "Nuevo reporte en tu zona de SIPINNA"
	message := "Se registró un reporte nuevo en tu zona de SIPINNA. Entra a la plataforma para revisarlo."
	if urgent {
		subject = "URGENTE: reporte con peligro inmediato en tu zona de SIPINNA"
		message = "Se registró un reporte en tu zona que indica que una niña, niño o adolescente está en peligro inmediato. Tiene prioridad alta: revísalo y activa el protocolo correspondiente."
	}

	link := strings.TrimRight(cfg.FrontendURL, "/") + "/reports"
	body, err := renderReportNotificationEmail(greeting, message, link, urgent)
	if err != nil {
		return err
	}

	text := fmt.Sprintf("%s,\n\n%s\n\nRevisar reportes: %s\n", greeting, message, link)

	return send(ctx, cfg, email, subject, body, text)
}

// SendReportReceipt envía el folio y la clave de consulta al ciudadano.
func SendReportReceipt(ctx context.Context, cfg *config.Config, email, folio, accessKey string) error {
	link := strings.TrimRight(cfg.FrontendURL, "/") + "/consultar-folio?" + url.Values{
		"folio": {folio},
		"clave": {accessKey},
	}.Encode()

	body, err := renderReportReceiptEmail(folio, accessKey, link)
	if err != nil {
		return err
	}

	text := fmt.Sprintf("¡Gracias por ayudar a proteger los derechos de niñas, niños y adolescentes!\n\nTu reporte fue recibido correctamente.\nFolio: %s\nClave de consulta: %s\n\nConsulta el avance: %s\n", folio, accessKey, link)

	return send(ctx, cfg, email, "Tu reporte RIETI "+folio, body, text)
}
