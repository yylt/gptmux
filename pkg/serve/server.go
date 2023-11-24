package serve

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/yylt/chatmux/pkg"
	"k8s.io/klog/v2"

	"github.com/gin-contrib/sse"
)

type Serve struct {
	e *gin.Engine

	// cache storage
	bk pkg.Backender

	mu sync.RWMutex
	// model map
	model map[string]pkg.PromptType
}

func NewServe(bk pkg.Backender) *Serve {
	s := &Serve{
		e:  gin.Default(),
		bk: bk,
	}
	s.probe()
	return s
}

func (s *Serve) probe() {
	group := s.e.Group("/api")

	group.GET("/chat/:id", s.idchatHandler)
	group.POST("/generate/id", s.genidHandler)

	v1group := s.e.Group("/v1")
	v1group.POST("/chat/completions", s.chatHandler) // chatbot ui
	v1group.GET("/models", s.modelsHandler)          // chatbot ui
}

func (s *Serve) Run(addr string) error {
	return s.e.Run(addr)
}

// not implement
func (s *Serve) idchatHandler(c *gin.Context) {
	_ = c.Param("id")
	c.AbortWithError(http.StatusNotAcceptable, http.ErrNotSupported)
}

// not implement
func (s *Serve) genidHandler(c *gin.Context) {
	c.AbortWithError(http.StatusNotAcceptable, http.ErrNotSupported)
}

// chat
func (s *Serve) chatHandler(c *gin.Context) {
	var (
		buf     = pkg.GetBuf()
		err     error
		message = pkg.ChatReq{}
	)
	defer pkg.PutBuf(buf)

	err = c.BindJSON(&message)
	if err != nil {
		klog.Error(err)
		return
	}

	s.mu.RLock()
	promptkind, ok := s.model[message.Model]
	if !ok {
		err = fmt.Errorf("not support model: %s", message.Model)
		c.AbortWithError(http.StatusBadRequest, err) //nolint: errcheck
		return
	}
	s.mu.RUnlock()

	for _, msg := range message.Messages {
		if msg == nil {
			continue
		}
		klog.Infof("chat by %s : %s", msg.Role, msg.Content)
		buf.WriteString(msg.Content)
	}

	readch, err := s.bk.Send(buf.String(), promptkind)
	if err != nil {
		klog.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err) //nolint: errcheck
		return
	}

	c.Stream(func(w io.Writer) bool {
		buf.Reset()

		select {
		case data, ok := <-readch:
			if !ok {
				cont := pkg.GetContent(&message, true, data.Err.Error())
				c.SSEvent("message", cont)
				return false
			}
			if data != nil {
				if data.Content != "" {
					buf.WriteString(data.Content)
				}
				if data.Err != nil {
					cont := pkg.GetContent(&message, true, data.Err.Error())
					c.SSEvent("message", cont)
					return false
				}
			}
		}

		cont := pkg.GetContent(&message, false, buf.String())
		c.SSEvent("message", cont)
		return true
	})

}

// model list
func (s *Serve) modelsHandler(c *gin.Context) {

	models := s.bk.Model()
	var (
		data = make([]*pkg.Model, len(models))
	)
	s.mu.Lock()
	for i, m := range models {
		data[i] = pkg.GetModel(m)
		s.model[data[i].Id] = m
	}
	s.mu.Unlock()
	c.JSON(200, map[string]interface{}{
		"object": "list",
		"data":   data,
	})
}

// sse event into io.reader
// io.reader consumed by user define function
func streamData(evs []sse.Event, fn func(io.Reader) error) error {
	buf := pkg.GetBuf()
	for _, ev := range evs {
		if v, ok := ev.Data.(string); ok {
			if v != "" && v != "[DONE]" {
				buf.WriteString(v)
			}
		}
	}
	defer pkg.PutBuf(buf)
	if buf.Len() > 0 {
		return fn(buf)
	}
	return io.EOF
}
