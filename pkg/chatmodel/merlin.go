package chatmodel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/yylt/gptmux/internal/database"
	"github.com/yylt/gptmux/internal/instance/model"
	"github.com/yylt/gptmux/mux/merlin"
	"github.com/yylt/gptmux/pkg/util"
)

const (
	merlinAppUrl    = "https://uam.getmerlin.in"
	merlinAccessUrl = "https://uam.getmerlin.in/session/get"
)

var _ einomodel.ToolCallingChatModel = (*Merlin)(nil)

type Merlin struct {
	mu             sync.Mutex
	authurl, proxy string
	debug          bool
	bes            map[string]*merlin.Instance
	db             database.Dababase
}

func (m *Merlin) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	var (
		msg             = &schema.Message{}
		lastUserContent string
		err             error
	)

	commonOptions := einomodel.GetCommonOptions(&einomodel.Options{}, opts...)
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.User {
			lastUserContent = input[i].Content
			break
		}
	}
	if commonOptions.Model == nil {
		commonOptions.Model = util.StringPtr("")
	}
	err = m.retryApiKey(func(be *merlin.Instance, modelType model.Capabilitie) error {
		switch modelType {
		case model.TextToText:
			return be.Chat(ctx, *commonOptions.Model, lastUserContent, func(done bool, content []byte) {
				msg.Content += string(content)
				if done {
					msg.Role = schema.Assistant
				}
			})
		case model.TextToImage:
			return be.Image(ctx, *commonOptions.Model, lastUserContent, func(done bool, content []byte) {
				msg.Content += string(content)
				if done {
					msg.Role = schema.Assistant
				}
			})
		default:
			return fmt.Errorf("unsupported model type for merlin: %s", modelType)
		}
	})
	return msg, err
}

func (m *Merlin) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	var (
		msg             = &schema.Message{}
		lastUserContent string
	)
	sr, sw := schema.Pipe[*schema.Message](3)

	commonOptions := einomodel.GetCommonOptions(&einomodel.Options{}, opts...)
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.User {
			lastUserContent = input[i].Content
			break
		}
	}
	if commonOptions.Model == nil {
		commonOptions.Model = util.StringPtr("")
	}
	go func() {
		defer sw.Close()
		err := m.retryApiKey(func(be *merlin.Instance, modelType model.Capabilitie) error {
			switch modelType {
			case model.TextToText:
				return be.Chat(ctx, *commonOptions.Model, lastUserContent, func(done bool, content []byte) {
					sw.Send(&schema.Message{
						Content: string(content),
						Role:    schema.Assistant,
					}, nil)
					msg.Content += string(content)
				})
			case model.TextToImage:
				return be.Image(ctx, *commonOptions.Model, lastUserContent, func(done bool, content []byte) {
					sw.Send(&schema.Message{
						Content: string(content),
						Role:    schema.Assistant,
					}, nil)
					msg.Content += string(content)
				})
			default:
				return fmt.Errorf("unsupported model type for merlin: %s", modelType)
			}
		})
		if err != nil {
			log.Printf("merlin stream error: %v", err)
			return
		}
	}()
	return sr, nil
}

func (m *Merlin) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

func (m *Merlin) getReadyOne(key string) (*merlin.Instance, string) {

	var (
		user, pass string
		err        error
	)
	m.mu.Lock()
	defer m.mu.Unlock()
	be, ok := m.bes[key]
	if !ok {
		if num := strings.IndexByte(key, ':'); num < 1 {
			log.Printf("invalid api key %s, should be in format user:password", key)
			return nil, key
		} else {
			user = strings.TrimSpace(key[:num])
			pass = strings.TrimSpace(key[num+1:])
		}
		be, err = merlin.NewInstance2(&merlin.InstanceConfig{
			User:     user,
			Password: pass,
			Proxy:    m.proxy,
			Debug:    m.debug,
			Authurl:  m.authurl,
		})
		if err != nil {
			log.Printf("create merlin instance for %s failed: %v", key, err)
			return nil, key
		}
		m.bes[key] = be
	}
	if !be.TryLock() {
		return nil, ""
	}
	return be, ""
}

func (m *Merlin) retryApiKey(fn func(be *merlin.Instance, modelType model.Capabilitie) error, opts ...einomodel.Option) error {
	var (
		opt        model.CustomOptions
		err        error
		invalidKey []string
	)
	einomodel.GetImplSpecificOptions(&opt, opts...)

	for _, key := range opt.Apikeys {

		be, key := m.getReadyOne(key)
		if be == nil {
			if key != "" {
				invalidKey = append(invalidKey, key)
			}
			continue
		}
		be.Lock()
		err = fn(be, opt.ModelType)
		be.Unlock()
		if err != nil {
			log.Printf("retry api key %s failed: %v", key, err)
			if errors.Is(err, merlin.ErrAuth) {
				invalidKey = append(invalidKey, key)
			}
		} else {
			break
		}
	}
	_ = m.db.MarkApiKey(false, invalidKey...)
	return err
}

func NewMerlin(cfg model.Instance, db database.Dababase) *Merlin {

	gem := &Merlin{
		authurl: cfg.Address,
		proxy:   cfg.Proxy,
		debug:   cfg.Debug,
		bes:     make(map[string]*merlin.Instance),
		db:      db,
	}
	return gem

}
