package chatmodel

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/yylt/gptmux/internal/database"
	"github.com/yylt/gptmux/internal/instance/model"
	"github.com/yylt/gptmux/pkg/util"
)

var _ einomodel.ToolCallingChatModel = (*Openai)(nil)

type Openai struct {
	*openai.ChatModel

	name string
	// config in chatModel inner.
	clicfg *openai.ChatModelConfig

	db database.Dababase
}

func (o *Openai) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	var (
		msg *schema.Message
		err error
	)
	err = o.retryApiKey(func(apikey string) error {
		o.clicfg.APIKey = apikey
		msg, err = o.ChatModel.Generate(ctx, input, opts...)
		if err != nil {
			return err
		}
		return nil
	}, opts...)

	return msg, err
}

func (o *Openai) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	var (
		msg *schema.StreamReader[*schema.Message]
		err error
	)
	err = o.retryApiKey(func(apikey string) error {
		o.clicfg.APIKey = apikey
		msg, err = o.ChatModel.Stream(ctx, input, opts...)
		if err != nil {
			return err
		}
		return nil
	}, opts...)

	return msg, err
}

func (o *Openai) retryApiKey(fn func(apikey string) error, opts ...einomodel.Option) error {
	var (
		opt        model.CustomOptions
		err        error
		invalidKey []string
	)
	einomodel.GetImplSpecificOptions(&opt, opts...)
	for _, key := range opt.Apikeys {
		err = fn(key)
		if err != nil {
			log.Printf("retry api key %s failed: %v", key, err)
			invalidKey = append(invalidKey, key)
		} else {
			break
		}
	}
	_ = o.db.MarkApiKey(false, invalidKey...)
	return err
}

func (o *Openai) GetType() string {
	return o.name
}

func NewOpenaiCompatible(cfg model.Instance, db database.Dababase) *Openai {
	var (
		cliconfig = openai.ChatModelConfig{
			APIKey: "fake",
		}
		err error
	)
	// Address Must set.
	cliconfig.BaseURL = cfg.Address
	cliconfig.HTTPClient, err = util.NewDebugHTTPClient(cfg.Proxy, cfg.Debug)
	if err != nil {
		log.Fatalf("failed to create OpenAI client: %v", err)
	}
	chatmodel, err := openai.NewChatModel(context.TODO(), &cliconfig)
	if err != nil {
		log.Fatalf("failed to create OpenAI chat model: %v", err)
	}
	gem := &Openai{
		db:        db,
		name:      cfg.Name,
		clicfg:    &cliconfig,
		ChatModel: chatmodel,
	}
	return gem
}
