package main

// This entry point contains only the service runtime.
// Upstream CLI utilities remain available from cmd/sing-box.

import (
	"context"
	"flag"
	"os"
	"os/signal"
	runtimeDebug "runtime/debug"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
	}

	flags := flag.NewFlagSet("sing-box-router", flag.ContinueOnError)
	configPath := flags.String("c", "config.json", "configuration file")
	workingDirectory := flags.String("D", "", "working directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return flag.ErrHelp
	}
	if *workingDirectory != "" {
		if err := os.Chdir(*workingDirectory); err != nil {
			return err
		}
	}

	globalCtx := context.Background()
	globalCtx = service.ContextWith(globalCtx, deprecated.NewStderrManager(log.StdLogger()))
	globalCtx = include.Context(globalCtx)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	for {
		instance, cancel, err := create(globalCtx, *configPath)
		if err != nil {
			return err
		}
		runtimeDebug.FreeOSMemory()

		received := <-signals
		cancel()
		if err = instance.Close(); err != nil {
			log.Error(err)
		}
		if received != syscall.SIGHUP {
			return nil
		}
	}
}

func create(ctx context.Context, configPath string) (*box.Box, context.CancelFunc, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	options, err := json.UnmarshalExtendedContext[option.Options](ctx, content)
	if err != nil {
		return nil, nil, err
	}
	instanceCtx, cancel := context.WithCancel(ctx)
	instance, err := box.New(box.Options{
		Context: instanceCtx,
		Options: options,
	})
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err = instance.Start(); err != nil {
		cancel()
		return nil, nil, err
	}
	return instance, cancel, nil
}
