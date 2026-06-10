package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

const defaultProjectId = "00000000-0000-4000-8000-000000000001"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cachebench <generate|run|table> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "table":
		err = cmdTable(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cachebench:", err)
		os.Exit(1)
	}
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	corpusDir := fs.String("corpus-dir", "", "directory holding the generated corpus (acts as local storage root)")
	projectId := fs.String("project-id", defaultProjectId, "project uuid used in storage keys")
	entries := fs.Int("entries", 1000, "number of bundle+map+tw triples to generate")
	tokens := fs.Int("tokens", 40000, "source map tokens per bundle (drives resolver size)")
	workers := fs.Int("workers", runtime.NumCPU()*2, "concurrent writers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *corpusDir == "" {
		return fmt.Errorf("--corpus-dir is required")
	}
	pid, err := uuid.Parse(*projectId)
	if err != nil {
		return err
	}
	config.Init(&config.Cfg{StorageType: "local", StoragePath: *corpusDir})
	if err := storage.Init(); err != nil {
		return err
	}
	start := time.Now()
	if err := generateCorpus(context.Background(), *corpusDir, pid, *entries, *tokens, *workers); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "generated %d entries in %s\n", *entries, time.Since(start).Round(time.Second))
	return nil
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfg := runConfig{}
	fs.StringVar(&cfg.corpusDir, "corpus-dir", "", "corpus directory from generate")
	fs.StringVar(&cfg.projectId, "project-id", defaultProjectId, "project uuid used in storage keys")
	fs.StringVar(&cfg.label, "label", "", "label for this cell in the results table (defaults to mode)")
	fs.StringVar(&cfg.mode, "mode", "memory", "cache mode: memory or disk")
	fs.IntVar(&cfg.entries, "entries", 1000, "corpus entries the workload draws from")
	fs.IntVar(&cfg.hot, "hot", 30, "distinct bundles in the hot rotation")
	fs.Float64Var(&cfg.coldRatio, "cold-ratio", 0.05, "fraction of lookups hitting a uniformly random bundle")
	fs.DurationVar(&cfg.duration, "duration", 60*time.Second, "measured duration")
	fs.DurationVar(&cfg.warmup, "warmup", 10*time.Second, "warmup excluded from stats")
	fs.IntVar(&cfg.concurrency, "concurrency", 16, "concurrent resolver workers")
	fs.IntVar(&cfg.memCacheMB, "mem-cache-mb", 0, "memory mode resolver cache budget in MB (0 = unbounded)")
	fs.IntVar(&cfg.openEntries, "open-entries", 512, "disk mode: max open mmap resolvers")
	fs.IntVar(&cfg.openMB, "open-mb", 1024, "disk mode: byte budget of the open-resolver layer in MB")
	fs.StringVar(&cfg.diskDir, "disk-cache-dir", "", "disk mode: local tw cache dir (wiped at start)")
	fs.IntVar(&cfg.diskMB, "disk-cache-mb", 16384, "disk mode: tw cache capacity in MB")
	fs.StringVar(&cfg.tier, "tier", "local", "instance label embedded in results")
	fs.StringVar(&cfg.out, "out", "-", "result JSON path (- for stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.corpusDir == "" {
		return fmt.Errorf("--corpus-dir is required")
	}
	if cfg.mode == "disk" && cfg.diskDir == "" {
		return fmt.Errorf("--disk-cache-dir is required in disk mode")
	}
	if cfg.label == "" {
		cfg.label = cfg.mode
	}
	config.Init(&config.Cfg{StorageType: "local", StoragePath: cfg.corpusDir})
	if err := storage.Init(); err != nil {
		return err
	}
	return runBench(cfg)
}

func cmdTable(args []string) error {
	fs := flag.NewFlagSet("table", flag.ExitOnError)
	resultsDir := fs.String("results-dir", "", "directory of run result JSON files")
	out := fs.String("out", "-", "markdown output path (- for stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *resultsDir == "" {
		return fmt.Errorf("--results-dir is required")
	}
	table, err := renderTable(*resultsDir)
	if err != nil {
		return err
	}
	if *out == "" || *out == "-" {
		_, err = os.Stdout.WriteString(table)
		return err
	}
	return os.WriteFile(*out, []byte(table), 0o644)
}
