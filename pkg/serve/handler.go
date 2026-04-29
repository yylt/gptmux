package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"

	"github.com/yylt/gptmux/pkg/mux"
	"github.com/yylt/gptmux/pkg/mux/merlin"
	"k8s.io/klog/v2"
)

type Controller struct {
	ctx        context.Context
	debug      bool
	configPath string
	saveConfig func() error
	mu         sync.RWMutex

	chats []mux.Model
}

func NewController(ctx context.Context, debug bool, configPath string, saveConfig func() error, ms ...mux.Model) *Controller {
	models := make([]mux.Model, 0, len(ms))
	for i := range ms {
		klog.Infof("append upstream '%s', index '%d'", ms[i].Name(), ms[i].Index())
		models = append(models, ms[i])
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Index() > models[j].Index()
	})
	return &Controller{
		ctx:        ctx,
		debug:      debug,
		configPath: configPath,
		saveConfig: saveConfig,
		chats:      models,
	}
}

func (ca *Controller) Context() context.Context {
	return ca.ctx
}

func (ca *Controller) Debug() bool {
	return ca.debug
}

func (ca *Controller) Models() []mux.Model {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.chats
}

func (ca *Controller) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", ca.AnthropicMessages)
	mux.HandleFunc("/v1/chat/completions", ca.OpenAIChatCompletions)
	mux.HandleFunc("/api/", ca.API)
	return mux
}

func (ca *Controller) Merlin() *merlin.Merlin {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	for _, chat := range ca.chats {
		if m, ok := chat.(*merlin.Merlin); ok {
			return m
		}
	}
	return nil
}

func (ca *Controller) SaveConfig() error {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if ca.saveConfig == nil {
		return nil
	}
	return ca.saveConfig()
}

func (ca *Controller) OpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "openai api placeholder: not implemented", http.StatusNotImplemented)
}

func (ca *Controller) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "anthropic api placeholder: not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
