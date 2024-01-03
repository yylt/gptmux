package serve

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yylt/chatmux/pkg"
	"github.com/yylt/chatmux/pkg/util"
	"k8s.io/klog/v2"

	"unicode/utf8"
)

var (
	hua, _ = utf8.DecodeRuneInString("画")
)

type Serve struct {
	e *gin.Engine

	// cache storage
	bk pkg.Backender
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
		buf     = util.GetBuf()
		err     error
		message = pkg.ChatReq{}
	)
	defer util.PutBuf(buf)

	err = c.BindJSON(&message)
	if err != nil {
		klog.Error(err)
		return
	}

	model, ok := pkg.ModelName(message.Model)
	if !ok {
		err = fmt.Errorf("not support model: %s", message.Model)
		c.AbortWithError(http.StatusBadRequest, err) //nolint: errcheck
		return
	}

	msg := message.Messages[len(message.Messages)-1]
	klog.Infof("last prompt: %s", msg.Content)
	buf.WriteString(msg.Content)

	for _, v := range msg.Content {
		if v == hua {
			model = pkg.ImgModel
		}
		break
	}

	readch, err := s.bk.Send(buf.String(), model)
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
	c.JSON(200, map[string]interface{}{
		"object": "list",
		"data":   pkg.GetModels(),
	})
}
