package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

var (
	rcloneCmd                string
	errorRetryDelay          time.Duration
	remoteCheckPeriod        time.Duration
	localCheckPeriod         time.Duration
	localChangeDebounceDelay time.Duration
)

func init() {
	flag.StringVar(&rcloneCmd, "rclone", "rclone", "rclone command")
	flag.DurationVar(&errorRetryDelay, "error-retry-delay", time.Minute, "delay before retries on error")
	flag.DurationVar(&remoteCheckPeriod, "remote-check-period", time.Minute, "period for remote file system checks")
	flag.DurationVar(&localCheckPeriod, "local-check-period", time.Second, "period for local file system checks")
	flag.DurationVar(&localChangeDebounceDelay, "local-change-debounce-delay", 5*time.Second, "debounce delay after change detection")
}

type config struct {
	remotePath, localPath string
}

func parseArgs() (*config, error) {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		return nil, fmt.Errorf("invalid number of arguments")
	}
	return &config{
		remotePath: args[0],
		localPath:  args[1],
	}, nil
}

func main() {
	log.Println("Start")
	cfg, err := parseArgs()
	if err != nil {
		fmt.Printf("error: %s\nusage: rclone-autosync remote_name:remote_path local_path\n", err)
		os.Exit(1)
	}
	for {
		err := run(cfg)
		if err == nil {
			log.Println("Done")
			return
		}
		log.Printf("error: %s", err)
		time.Sleep(errorRetryDelay)
	}
}

func run(cfg *config) error {
	if err := syncDown(cfg); err != nil {
		return err
	}
	if err := syncUp(cfg); err != nil {
		return err
	}
	downSyncTicker := time.NewTicker(remoteCheckPeriod)
	defer downSyncTicker.Stop()
	localCheckTicker := time.NewTicker(localCheckPeriod)
	defer localCheckTicker.Stop()
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)
	var files []os.FileInfo
	var localChangeDetectedAt *time.Time
	log.Println("Watch file system")
	for {
		select {

		case now := <-localCheckTicker.C:
			newFiles, err := listAllFiles(cfg.localPath)
			if err != nil {
				return fmt.Errorf("list local files: %w", err)
			}
			if files != nil {
				if !areSameFiles(files, newFiles) {
					if localChangeDetectedAt == nil {
						log.Println("Local change detected")
					}
					localChangeDetectedAt = &now
				} else {
					if localChangeDetectedAt != nil {
						if now.Sub(*localChangeDetectedAt) >= localChangeDebounceDelay {
							if err := syncUp(cfg); err != nil {
								return err
							}
							localChangeDetectedAt = nil
						}
					}
				}
			}
			files = newFiles

		case <-downSyncTicker.C:
			if err := syncDown(cfg); err != nil {
				return err
			}

		case <-interruptChan:
			log.Println("Interrupt intercepted")
			return syncUp(cfg)
		}
	}
}

func sync(from, to string) error {
	cmd := exec.Command(rcloneCmd, `sync`, from, to, `--stats-log-level`, `DEBUG`, `-v`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func syncDown(cfg *config) error {
	log.Println("Sync down")
	if err := sync(cfg.remotePath, cfg.localPath); err != nil {
		return fmt.Errorf("sync down: %w", err)
	}
	return nil
}

func syncUp(cfg *config) error {
	log.Println("Sync up")
	if err := sync(cfg.localPath, cfg.remotePath); err != nil {
		return fmt.Errorf("sync up: %w", err)
	}
	return nil
}

func listAllFiles(path string) ([]fs.FileInfo, error) {
	var files []fs.FileInfo
	err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		files = append(files, info)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return files, nil
}

func areSameFiles(a, b []fs.FileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !isSameFile(a[i], b[i]) {
			return false
		}
	}
	return true
}

func isSameFile(a, b fs.FileInfo) bool {
	return a.Name() == b.Name() && a.Size() == b.Size() && a.Mode() == b.Mode() && a.ModTime() == b.ModTime()
}
