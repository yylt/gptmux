package serve

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yylt/chatmux/pkg"
	"k8s.io/klog/v2"

	"github.com/gin-contrib/sse"
)

type Serve struct {
	e *gin.Engine

	// cache storage
	bk pkg.Backender

	// model map
	// chatgpt id - local modelname
	model map[string]pkg.PromptType
}

func NewServe(bk pkg.Backender) *Serve {
	s := &Serve{
		e:     gin.Default(),
		bk:    bk,
		model: make(map[string]pkg.PromptType),
	}
	models := s.bk.Model()
	var (
		md = make([]*pkg.Model, len(models))
	)
	for i, m := range models {
		md[i] = pkg.GetModel(m)
		if md[i] == nil {
			panic(fmt.Errorf("not found model %v", m))
		}
		s.model[md[i].Id] = m
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

	promptkind, ok := s.model[message.Model]
	if !ok {
		err = fmt.Errorf("not support model: %s", message.Model)
		c.AbortWithError(http.StatusBadRequest, err) //nolint: errcheck
		return
	}

	for _, msg := range message.Messages {
		if msg == nil {
			continue
		}
		klog.Infof("prompt is role(%s): %s", msg.Role, msg.Content)
	}
	buf.WriteString(message.Messages[len(message.Messages)-1].Content)

	readch, err := s.bk.Send(buf.String(), promptkind)
	if err != nil {
		klog.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err) //nolint: errcheck
		return
	}

	c.Stream(func(w io.Writer) bool {
		buf.Reset()

		data, ok := <-readch
		if !ok {
			cont := pkg.GetContent(&message, true, "")
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

		cont := pkg.GetContent(&message, false, buf.String())
		c.SSEvent("message", cont)
		return true
	})

}

// model list
func (s *Serve) modelsHandler(c *gin.Context) {

	var (
		data = make([]*pkg.Model, len(s.model))
		i    = 0
	)

	for _, name := range s.model {
		data[i] = pkg.GetModel(name)
		i++
	}

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
