package main

import (
	"context"
	"flag"

	"github.com/yylt/gptmux/pkg/box"
	"github.com/yylt/gptmux/pkg/claude"
	"github.com/yylt/gptmux/pkg/merlin"
	"github.com/yylt/gptmux/pkg/serve"
)

func main() {

	cp := flag.String("config", "", "config file path")
	flag.Parse()
	if cp == nil || *cp == "" {
		panic("config must set")
	}
	cfg, err := LoadConfigmap(*cp)
	if err != nil {
		panic(err)
	}
	notify := box.New(&cfg.Notify)

	ctx := context.Background()
	ml := merlin.NewMerlinIns(&cfg.Merlin)
	ca := claude.New(ctx, &cfg.Claude, notify)

	server := serve.NewServe()
	cfg.Model.Initial(server, ml, ca)

	server.Run(cfg.Addr)
}
