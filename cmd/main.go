package main

import (
	"flag"

	"github.com/yylt/chatmux/pkg/merlin"
	"github.com/yylt/chatmux/pkg/serve"
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
	ml := merlin.NewMerlinIns(&cfg.Merlin)

	server := serve.NewServe(ml)

	server.Run(*&cfg.Addr)
}
