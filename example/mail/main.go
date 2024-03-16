package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gotify/go-api-client/v2/auth"
	"github.com/gotify/go-api-client/v2/client/message"
	"github.com/gotify/go-api-client/v2/gotify"
	"github.com/gotify/go-api-client/v2/models"
)

const (
	gotifyURL        = "x"
	applicationToken = "x"
	user             = "x"
	password         = "x"
)

func main() {
	myURL, _ := url.Parse(gotifyURL)
	client := gotify.NewClient(myURL, &http.Client{})
	versionResponse, err := client.Version.GetVersion(nil)

	if err != nil {
		log.Fatal("Could not request version ", err)
		return
	}
	version := versionResponse.Payload
	log.Println("Found version", *version)
	msgok, err := client.Message.GetMessages(message.NewGetMessagesParams(), auth.BasicAuth(user, password))
	if err != nil {
		log.Fatalf("message failed: %v", err)
	}
	for _, msg := range msgok.Payload.Messages {
		log.Println(msg.ApplicationID, ": ", msg.Message)
	}
	params := message.NewCreateMessageParams()
	params.Body = &models.MessageExternal{
		Title:    "my title",
		Message:  "my message",
		Priority: 5,
	}

	//_, err = client.Message.CreateMessage(params, auth.TokenAuth(applicationToken))
	if err != nil {
		log.Fatalf("Could not send message %v", err)
		return
	}
	log.Println("Message Sent!")
}
