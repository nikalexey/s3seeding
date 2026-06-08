// Command s3seeding
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"s3bench/internal/config"
	"s3bench/internal/s3client"
	"s3bench/internal/seed"
)

func main() {
	log.SetFlags(log.LstdFlags)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "path to JSON config")
	storageName := fs.String("storage", "zakroma", "storage name for the seed command")
	workers := fs.Int("workers", 8, "number of parallel uploaders (seed)")
	maxObjects := fs.Int64("max-objects", 0, "stop after N objects (seed; 0 = unlimited)")
	maxBytes := fs.Int64("max-bytes", 0, "stop after N bytes (seed; 0 = unlimited)")
	_ = fs.Parse(os.Args[2:])

	switch cmd {
	case "seed":
		runSeed(*cfgPath, *storageName, *workers, *maxObjects, *maxBytes)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `s3seeding — benchmark comparing S3 storages

Usage:
  s3seeding <command> [flags]

Commands:
  seed    continuously upload random data into a single storage (Ctrl-C to stop)

Flags:
  -config       path to JSON config (default config.json)
  -storage      storage name for seed
  -workers      number of parallel uploaders for seed (default 8)
  -max-objects  stop seed after N objects (0 = unlimited)
  -max-bytes    stop seed after N bytes (0 = unlimited)
`)
}

func runSeed(cfgPath, storageName string, workers int, maxObjects, maxBytes int64) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	st, ok := findStorage(cfg, storageName)
	if !ok {
		log.Fatalf("storage %q not found in config", storageName)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := s3client.New(ctx, st, cfg.PartSizeMB)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("seeding storage %q (%s), workers=%d. Ctrl-C to stop.", st.Name, st.Endpoint, workers)

	sd := seed.New(client, seed.Options{
		Workers:    workers,
		MaxObjects: maxObjects,
		MaxBytes:   maxBytes,
		Dist:       seed.FromConfig(cfg.Seed),
	})
	sd.Run(ctx)
}

func findStorage(cfg *config.Config, name string) (config.Storage, bool) {
	for _, s := range cfg.Storages {
		if s.Name == name {
			return s, true
		}
	}
	return config.Storage{}, false
}
