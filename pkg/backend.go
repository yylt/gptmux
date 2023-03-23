package pkg

import (
	"io"
)

type PromptType int

const (
	Unknown PromptType = 0
	All     PromptType = 1
	Text    PromptType = 2
	Image   PromptType = 3
	Video   PromptType = 4
)

type Backender interface {
	// 发送文本
	SendText(prompt io.Reader) (io.Reader, error)
	// 可用额度
	Allocate() int
	// 支持类型
	Support() []PromptType
	// 支持类型
	IsSupport(PromptType) bool
}

// TODO manage backend
type Backends struct {
	types map[PromptType][]Backender
}

func PickOne(bs []Backender, t PromptType) Backender {
	for _, bk := range bs {
		if bk.IsSupport(t) && bk.Allocate() > 0 {
			return bk
		}
	}
	return nil
}
