package serve

import (
	"fmt"
	"io"
	"net/http"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"

	"unicode/utf8"
)

var (
	hua, _ = utf8.DecodeRuneInString("画")
)

type Conf struct {
	Gpt3 string `yaml:"gpt3,omitempty"`
	Gpt4 string `yaml:"gpt4,omitempty"`
	Img  string `yaml:"image,omitempty"`
}

func (c *Conf) Initial(s *Serve, bks ...pkg.Backender) {
	var (
		bkindex = map[string]int{}
	)
	for i, v := range bks {
		bkindex[v.Name()] = i
	}

	v, ok := bkindex[c.Gpt3]
	if ok {
		s.bks[pkg.GPT3Model] = bks[v]
	}

	v, ok = bkindex[c.Gpt4]
	if ok {
		s.bks[pkg.GPT4Model] = bks[v]
	}
	v, ok = bkindex[c.Img]
	if ok {
		s.bks[pkg.ImgModel] = bks[v]
	}
}

type Serve struct {
	e *gin.Engine

	bks map[pkg.ChatModel]pkg.Backender
}

func NewServe() *Serve {

	s := &Serve{
		e:   gin.Default(),
		bks: map[pkg.ChatModel]pkg.Backender{},
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
	bk, ok := s.bks[model]
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("not found backend")) //nolint: errcheck
		return
	}

	// TODO, last prompt
	msg := message.Messages[len(message.Messages)-1]

	first, _ := utf8.DecodeRuneInString(msg.Content)
	if first == hua {
		model = pkg.ImgModel
	} else {
		if !unicode.Is(unicode.Han, first) {
			buf.WriteString("使用中文,")
		}
	}

	buf.WriteString(msg.Content)

	bufstr := buf.String()

	klog.Infof("backend: %s, prompt: %s", bk.Name(), bufstr)

	readch, err := bk.Send(bufstr, model)
	if err != nil {
		klog.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err) //nolint: errcheck
		return
	}

	c.Stream(func(w io.Writer) bool {
		buf.Reset()

		data, ok := <-readch
		if !ok {
			c.SSEvent("message", "[DONE]")
			return false
		}
		if data != nil {
			if data.Content != "" {
				buf.WriteString(data.Content)
			}
			if data.Err != nil {
				c.SSEvent("message", "[DONE]")
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
