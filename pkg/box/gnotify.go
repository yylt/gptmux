package box

import (
	"net/http"
	"net/url"

	"github.com/gotify/go-api-client/v2/auth"
	"github.com/gotify/go-api-client/v2/client"
	"github.com/gotify/go-api-client/v2/client/message"
	"github.com/gotify/go-api-client/v2/gotify"
	"github.com/gotify/go-api-client/v2/models"
	"k8s.io/klog/v2"
)

var _ Box = &gtify{}

type gtifyconf struct {
	Apptoken string `yaml:"app_token"`

	Appid int64 `yaml:"app_id,omitempty"`

	Server string `yaml:"server"`

	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type gtify struct {
	cf *gtifyconf

	cli *client.GotifyREST
}

func newGnotify(cf *gtifyconf) *gtify {
	if cf == nil {
		return nil
	}
	myURL, err := url.Parse(cf.Server)
	if err != nil {
		klog.Errorf("gnotify url %s parse failed: %v", cf.Server, err)
		return nil
	}

	client := gotify.NewClient(myURL, &http.Client{})
	_, err = client.Version.GetVersion(nil)
	if err != nil {
		klog.Errorln("Could not request version: ", err)
		return nil
	}

	return &gtify{
		cf:  cf,
		cli: client,
	}
}
func (g *gtify) Name() string {
	return "gnotify"
}
func (g *gtify) Push(ms *Message) error {
	params := message.NewCreateMessageParams()
	params.Body = &models.MessageExternal{
		Title:    ms.Title,
		Message:  ms.Msg,
		Priority: 5,
	}
	_, err := g.cli.Message.CreateMessage(params, auth.TokenAuth(g.cf.Apptoken))
	return err
}

func (g *gtify) Receive(fn func(*Message) bool) error {
	var (
		msgs []*models.MessageExternal
		lom  = &Message{}
	)
	authinfo := auth.BasicAuth(g.cf.User, g.cf.Password)
	if g.cf.Appid != 0 {
		msgok, err := g.cli.Message.GetAppMessages(message.NewGetAppMessagesParams().WithID(g.cf.Appid), authinfo)
		if err != nil {
			return err
		}
		msgs = msgok.Payload.Messages
	} else {
		msgok, err := g.cli.Message.GetMessages(message.NewGetMessagesParams(), authinfo)
		if err != nil {
			return err
		}
		msgs = msgok.Payload.Messages
	}

	for _, m := range msgs {
		lom.Msg = m.Message
		lom.Title = m.Title
		lom.Time = m.Date
		ok := fn(lom)
		if !ok {
			return nil
		}
	}
	return nil
}
