package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dominicgisler/imap-spam-cleaner/app"
	"github.com/dominicgisler/imap-spam-cleaner/config"
	"github.com/dominicgisler/imap-spam-cleaner/imap"
	"github.com/dominicgisler/imap-spam-cleaner/inbox"
	"github.com/dominicgisler/imap-spam-cleaner/logx"
	"github.com/dominicgisler/imap-spam-cleaner/provider"
)

const name = "imap-spam-cleaner"

var version = "dev"
var options app.Options
var checkProviderName string

func init() {
	var v bool
	flag.BoolVar(&v, "version", false, "Show short version")
	flag.BoolVar(&options.RunNow, "now", false, "Run all inboxes once immediately, ignoring schedule")
	flag.BoolVar(&options.AnalyzeOnly, "analyze-only", false, "Only analyze emails, do not move or delete them")
	flag.StringVar(&checkProviderName, "check-provider", "", "Run a non-destructive connectivity check for one configured provider")
	flag.Parse()
	if v {
		fmt.Printf("%s %s\n", name, version)
		os.Exit(0)
	}
}

func main() {
	logx.Infof("Starting %s %s", name, version)
	c, err := config.Load()
	if err != nil {
		logx.Errorf("Could not load config: %v", err)
		return
	}

	ctx := app.Context{
		Config:  c,
		Options: options,
	}

	var p provider.Provider
	for name, prov := range c.Providers {
		p, err = provider.New(prov.Type)
		if err != nil {
			logx.Errorf("Could not load provider: %v\n", err)
			return
		}
		if err = p.ValidateConfig(prov.Config); err != nil {
			logx.Errorf("Invalid config for provider %s: %v\n", name, err)
			return
		}
	}

	if checkProviderName != "" {
		prov, ok := c.Providers[checkProviderName]
		if !ok {
			logx.Errorf("Unknown configured provider %s", checkProviderName)
			os.Exit(1)
		}
		p, err = provider.New(prov.Type)
		if err != nil {
			logx.Errorf("Could not load provider %s: %v", checkProviderName, err)
			os.Exit(1)
		}
		if err = p.Init(prov.Config); err != nil {
			logx.Errorf("Could not init provider %s: %v", checkProviderName, err)
			os.Exit(1)
		}
		score, analyzeErr := p.Analyze(imap.Message{
			From:     "healthcheck@localhost",
			To:       "healthcheck@localhost",
			Subject:  "Provider connectivity check",
			Contents: []string{"This is a harmless synthetic email used to verify the configured spam analysis provider."},
		})
		if analyzeErr != nil {
			logx.Errorf("Provider %s check failed: %v", checkProviderName, analyzeErr)
			os.Exit(1)
		}
		logx.Infof("Provider %s check succeeded with score %d/100", checkProviderName, score)
		return
	}

	if ctx.Options.RunNow {
		logx.Info("Running all inboxes once immediately")
		inbox.RunAllInboxes(ctx)
		return
	}

	inbox.Schedule(ctx)
}
