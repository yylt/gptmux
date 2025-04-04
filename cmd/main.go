package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	openapi "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/mux/claude"
	"github.com/yylt/gptmux/mux/deepseek"
	"github.com/yylt/gptmux/mux/merlin"
	"github.com/yylt/gptmux/mux/ollama"
	"github.com/yylt/gptmux/mux/openai"
	"github.com/yylt/gptmux/mux/rkllm"
	"github.com/yylt/gptmux/mux/zhipu"

	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	cp := flag.String("config", "", "config file path")
	flag.Parse()
	if cp == nil || *cp == "" {
		panic("config must set")
	}
	cfg, err := LoadConfigmap(*cp)
	if err != nil {
		panic(err)
	}
	ctx := SetupSignalHandler()

	var ms []mux.Model
	ds := deepseek.New(&cfg.Deepseek)
	if ds != nil {
		ms = append(ms, ds)
	}
	ml := merlin.NewMerlinIns(&cfg.Merlin)
	if ml != nil {
		ms = append(ms, ml)
	}
	zp := zhipu.New(&cfg.Zhipu)
	if zp != nil {
		ms = append(ms, zp)
	}
	apidp := openai.New(ctx, &cfg.DeepseekApi)
	if apidp != nil {
		ms = append(ms, apidp)
	}
	ca := claude.New(ctx, &cfg.Claude)
	if ca != nil {
		ms = append(ms, ca)
	}
	rk := rkllm.New(&cfg.Rkllm)
	if rk != nil {
		ms = append(ms, rk)
	}

	ollm := ollama.New(ctx, &cfg.Ollama)
	if ollm != nil {
		ms = append(ms, ollm)

	}
	sili := openai.New(ctx, &cfg.Silicon)
	if sili != nil {
		ms = append(ms, sili)
	}
	chat := NewController(ctx, cfg.Debug, ms...)

	muxhandler := openapi.ApiHandleFunctions{
		ChatAPI:        chat,
		CompletionsAPI: chat,
		ModelsAPI:      chat,
	}
	e := gin.Default()
	openapi.NewRouterWithGinEngine(e, muxhandler)

	e.Run(cfg.Addr)
}

func SetupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		cancel()
		os.Exit(0)
	}()

	return ctx
}
