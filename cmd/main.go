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

	"github.com/yylt/gptmux/mux/rkllm"

	"github.com/yylt/gptmux/mux/ollama"

	"github.com/yylt/gptmux/pkg/box"
	"github.com/yylt/gptmux/pkg/handler"
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
	b := box.New(&cfg.Notify)

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
	ca := claude.New(ctx, &cfg.Claude, b)
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
	mux := openapi.ApiHandleFunctions{
		ChatAPI:   handler.NewChat(ctx, ms...),
		ModelsAPI: handler.NewModel(ctx),
	}
	e := gin.Default()
	openapi.NewRouterWithGinEngine(e, mux)

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
