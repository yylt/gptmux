package serve

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
	"github.com/yylt/chatmux/pkg"

	"github.com/gin-contrib/sse"
)

type Serve struct {
	e *gin.Engine

	// cache storage
	store pkg.Storage
	bs    []pkg.Backender
}

type answerfn func(r io.Reader) error

const (
	UndefineId string = "undefined"
)

func NewServe(store pkg.Storage, bes []pkg.Backender) *Serve {
	r := gin.Default()
	rand.Seed(time.Now().UnixNano())

	s := &Serve{
		e:     r,
		store: store,
		bs:    bes,
	}
	s.probe()
	return s
}

func (s *Serve) probe() {
	group := s.e.Group("/api")
	group.GET("/conv/:id", s.AnswerList)
	group.GET("/chat/:id", s.GetAnswer)
	group.POST("/generate/id", s.GenId)
}

func (s *Serve) send(c *gin.Context, id string) (answerfn, error) {
	// TODO 根据contentType 分流，根据 用户 分流

	back := pkg.PickOne(s.bs, pkg.Text)
	if back == nil {
		return nil, fmt.Errorf("not found ava backend")
	}
	bucket, _ := s.store.GetBucket(id)

	return func(r io.Reader) error {
		var (
			rb, wb = pkg.GetBuf(), pkg.GetBuf()
		)
		defer func() {
			bucket.Set(rb.String(), wb.Bytes(), 24*time.Hour)
			pkg.PutBuf(rb)
			pkg.PutBuf(wb)
		}()
		read, err := back.SendText(io.TeeReader(r, rb))
		if err != nil {
			wb.WriteString("服务端出错")
			return err
		}
		scan := bufio.NewScanner(io.TeeReader(read, wb))
		scan.Split(bufio.ScanRunes)
		c.Stream(func(w io.Writer) bool {
			// Stream message to client from message channel
			for scan.Scan() {
				c.SSEvent("message", scan.Text())
				return true
			}
			return false
		})
		return nil
	}, nil
}

func (s *Serve) GenId(c *gin.Context) {
	id := xid.New()
	_, err := s.store.CreateBucket(id.String(), 24*time.Hour)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	c.JSON(200, NewGeneralResp(200, id.String()))
}

// 回答提示词，prompt有 query 和 sse 两类来源
// query 优先级高于 sse 的数据
func (s *Serve) GetAnswer(c *gin.Context) {
	var (
		events []sse.Event
		err    error
	)
	id := c.Param("id")
	if _, err = s.store.GetBucket(id); err != nil {
		c.AbortWithError(400, err)
		return
	}
	prompt := c.Query("prompt")
	if c.ContentType() == "text/event-stream" && prompt == "" {
		events, err = sse.Decode(c.Request.Body)
		if err != nil {
			c.AbortWithError(400, err)
			return
		}
	}
	fn, err := s.send(c, id)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	if prompt != "" {
		err = fn(bytes.NewBufferString(prompt))
	} else {
		err = streamData(events, fn)
	}
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
}

// 获取指定id的回答列表信息
func (s *Serve) AnswerList(c *gin.Context) {
	id := c.Param("id")
	buck, err := s.store.GetBucket(id)
	if err != nil {
		c.AbortWithError(400, err)
		return
	}
	var (
		resplist []string
	)
	buck.Iter(func(s1, s2 string) bool {
		resplist = append(resplist, s1, s2)
		return true
	})
	resp := NewListResp(id, "null", resplist...)
	c.JSON(200, NewGeneralResp(200, resp))
	return
}

// block unless error
func (s *Serve) Run(addr string) error {
	return s.e.Run(addr)
}

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
