package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/tamnd/singularity-agent/site"
)

func runSite(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: site needs a subcommand: build, check or serve", errUsage)
	}
	switch args[0] {
	case "build":
		return siteBuild(args[1:], stdout, stderr)
	case "check":
		return siteCheck(args[1:], stdout, stderr)
	case "serve":
		return siteServe(args[1:], stdout, stderr)
	}
	return fmt.Errorf("%w: unknown subcommand %q; use build, check or serve", errUsage, args[0])
}

func siteBuild(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing site build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", "../singularity", "the root of the content repository")
	out := fs.String("out", "public", "the output directory; the build empties it first")
	url := fs.String("url", "https://singularity.tamnd.com", "the public address of the site")
	commit := fs.String("commit", "", "the commit of the content repository")
	date := fs.String("date", "", "the build date, for example 2026-11-23")
	check := fs.Bool("check", true, "check the site after the build")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: build takes no arguments", errUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := site.Build(ctx, site.Options{Content: *content, Out: *out, URL: *url, Commit: *commit, Date: *date, Version: Version})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "built %d pages, %d files, %d bytes in %s\n", res.Pages, res.Files, res.Bytes, *out)
	if *check {
		return report(*out, stdout)
	}
	return nil
}

func siteCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing site check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "public", "the built site")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	return report(*dir, stdout)
}

func report(dir string, stdout io.Writer) error {
	r, err := site.Check(dir)
	if err != nil {
		return err
	}
	for _, p := range r.Problems {
		fmt.Fprintln(stdout, "problem:", p)
	}
	if !r.OK() {
		return fmt.Errorf("the site has %d problems", len(r.Problems))
	}
	fmt.Fprintf(stdout, "the site is good: %d files, %d bytes, in the limits of Cloudflare static assets\n", r.Files, r.Bytes)
	return nil
}

func siteServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing site serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "public", "the built site")
	addr := fs.String("addr", "127.0.0.1:8080", "the address to listen on")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	h, err := site.Handler(*dir)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "serving %s at http://%s/\n", *dir, ln.Addr())
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
