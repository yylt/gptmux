package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/yylt/gptmux/pkg/mux"
	"github.com/yylt/gptmux/pkg/mux/merlin"
	"github.com/yylt/gptmux/pkg/serve"

	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	cp := flag.String("config", "", "config file path")
	flag.Parse()
	logBuildInfo()
	if cp == nil || *cp == "" {
		panic("config must set")
	}
	cfg, err := LoadConfigmap(*cp)
	if err != nil {
		panic(err)
	}
	ctx := SetupSignalHandler()

	var ms []mux.Model
	ml := merlin.NewMerlinIns(&cfg.Merlin)
	if ml != nil {
		ms = append(ms, ml)
	}
	ctl := serve.NewController(ctx, cfg.Debug, *cp, func() error {
		return cfg.SaveConfig(*cp)
	}, ms...)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: ctl.Handler(),
	}

	go func() {
		klog.Infof("listen on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Exitf("listen failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("shutdown failed: %v", err)
	}
}

func logBuildInfo() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		klog.Info("build info unavailable")
		return
	}

	settings := map[string]string{}
	for _, s := range bi.Settings {
		settings[s.Key] = s.Value
	}

	klog.Infof(
		"build info: module=%s version=%s go=%s commit=%s time=%s modified=%s",
		bi.Main.Path,
		bi.Main.Version,
		bi.GoVersion,
		settings["vcs.revision"],
		settings["vcs.time"],
		settings["vcs.modified"],
	)
	if bi.Main.Sum != "" {
		klog.Infof("build checksum: %s", bi.Main.Sum)
	}
	for _, dep := range bi.Deps {
		if dep.Path == "" {
			continue
		}
		klog.V(2).Infof("dep: %s@%s", dep.Path, dep.Version)
	}
}

func SetupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	return ctx
}
