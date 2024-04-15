package mux

import (
	"github.com/tmc/langchaingo/llms"
)

type Model interface {
	llms.Model

	Name() string
}
