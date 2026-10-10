// Package resend envía correos electrónicos a través de la API de Resend.
package resend

import (
	"fmt"
	"go-service-sipinna/internal/config"

	"github.com/resend/resend-go/v4"
)

// SendMessage envía un correo a recipientEmail desde no-reply@sipinna.com usando la
// API key de cfg. Por ahora el asunto y el contenido son fijos (de prueba).
//
// Es síncrona y no regresa error: si el envío falla solo lo imprime en la salida
// estándar.
func SendMessage(cfg *config.Config, recipientEmail string) {
	client := resend.NewClient(cfg.SipinnaResendAPIKey)

	params := &resend.SendEmailRequest{
		From:    "SIPINNA <no-reply@sipinna.com>",
		To:      []string{recipientEmail},
		Html:    "<strong>hello world</strong>",
		Subject: "Hello from Golang",
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(sent.Id)
}
