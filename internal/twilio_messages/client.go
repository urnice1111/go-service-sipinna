package twilio_messages

import (
	"fmt"
	"go-service-sipinna/internal/config"

	"github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

func GetTwilioClient(cfg *config.Config) *twilio.RestClient {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username:   cfg.TwilioApiKey,
		Password:   cfg.TwilioApiSecret,
		AccountSid: cfg.TwilioAccountSID,
	})
	return client
}

func SendSMS(client *twilio.RestClient, textBody string, toNumber string) error {
	params := &api.CreateMessageParams{}
	params.SetBody(textBody)
	params.SetFrom("+17402848974")
	params.SetTo(toNumber)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return err
	} else {
		if resp.Body != nil {
			fmt.Println(*resp.Body)
		} else {
			fmt.Println(resp.Body)
		}
	}
	return nil
}
