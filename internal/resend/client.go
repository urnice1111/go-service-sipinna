package resend

import (
	"fmt"
	"go-service-sipinna/internal/config"

	"github.com/resend/resend-go/v4"
)

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
