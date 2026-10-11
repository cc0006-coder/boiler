package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/rishiyaduwanshi/boiler/internal/config"
	"github.com/rishiyaduwanshi/boiler/internal/utils"
)

func setupCommandStore(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	homeKey := "HOME"
	if runtime.GOOS == "windows" {
		homeKey = "USERPROFILE"
	}
	t.Setenv(homeKey, home)
	previousLogger := logger
	t.Cleanup(func() {
		logger = previousLogger
		if Cmd.Flags().Lookup("force") != nil {
			if err := Cmd.Flags().Set("force", "false"); err != nil {
				t.Errorf("reset force: %v", err)
			}
		}
	})
	var err error
	logger, err = utils.NewLogger(filepath.Join(home, "logs"), false)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(home, "routes.bl")
	if err := os.WriteFile(source, []byte("new command\n"), 0644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(config.DefaultConfig().Paths.Commands, "routes.bl")
	return source, destination
}

func TestStoreCommandCreatesNewFile(t *testing.T) {
	source, destination := setupCommandStore(t)
	if err := storeCommand(source); err != nil {
		t.Fatal(err)
	}
	assertCommandContent(t, destination, "new command\n")
}

func TestStoreCommandRefusesOverwrite(t *testing.T) {
	source, destination := setupCommandStore(t)
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("original command\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := storeCommand(source)
	if err == nil || !strings.Contains(err.Error(), "Use --force to overwrite") {
		t.Errorf("expected overwrite refusal, got %v", err)
	}
	assertCommandContent(t, destination, "original command\n")
}

func TestStoreCommandForceOverwrite(t *testing.T) {
	source, destination := setupCommandStore(t)
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("original command\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Cmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	if err := storeCommand(source); err != nil {
		t.Fatal(err)
	}
	assertCommandContent(t, destination, "new command\n")
}

func TestStoreCommandRefusesExistingDirectory(t *testing.T) {
	source, destination := setupCommandStore(t)
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	err := storeCommand(source)
	if err == nil || !strings.Contains(err.Error(), "Use --force to overwrite") {
		t.Fatalf("expected destination refusal, got %v", err)
	}
	if !utils.IsDirectory(destination) {
		t.Fatal("existing directory was changed")
	}
}

func TestStoreCommandForceRefusesSameFile(t *testing.T) {
	source, destination := setupCommandStore(t)
	if err := storeCommand(source); err != nil {
		t.Fatal(err)
	}
	if err := Cmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	err := storeCommand(destination)
	if err == nil || !strings.Contains(err.Error(), "same file") {
		t.Errorf("expected same-file refusal, got %v", err)
	}
	assertCommandContent(t, destination, "new command\n")
}

func TestStoreCommandRejectsInvalidSource(t *testing.T) {
	for _, name := range []string{"missing.bl", "plain.txt", "directory.bl"} {
		t.Run(name, func(t *testing.T) {
			source, destination := setupCommandStore(t)
			invalid := filepath.Join(filepath.Dir(source), name)
			if name == "plain.txt" {
				if err := os.WriteFile(invalid, []byte("text"), 0644); err != nil {
					t.Fatal(err)
				}
			} else if name == "directory.bl" {
				if err := os.Mkdir(invalid, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := storeCommand(invalid); err == nil {
				t.Fatal("expected invalid source error")
			}
			if utils.FileExists(filepath.Dir(destination)) {
				t.Fatal("invalid source created a commands directory")
			}
		})
	}
}

func assertCommandContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("command content = %q, want %q", content, want)
	}
}

func TestStoreCommandConcurrentCreation(t *testing.T) {
	_, destination := setupCommandStore(t)
	const workers = 32
	sources := make([]string, workers)
	for i := range sources {
		dir := t.TempDir()
		sources[i] = filepath.Join(dir, "routes.bl")
		if err := os.WriteFile(sources[i], []byte(strings.Repeat("command\n", 128*1024)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for _, source := range sources {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			<-start
			results <- storeCommand(path)
		}(source)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !strings.Contains(err.Error(), "Use --force to overwrite") {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful stores = %d, want 1", successes)
	}
	assertCommandContent(t, destination, strings.Repeat("command\n", 128*1024))
}

// Execute the Cobra command in a child process because its Run function calls os.Exit.
func TestStoreCommandCLIProcess(t *testing.T) {
	if os.Getenv("BOILER_STORE_CLI_TEST") != "1" {
		return
	}
	log, err := utils.NewLogger(filepath.Join(os.Getenv("BOILER_STORE_TEST_HOME"), "logs"), false)
	if err != nil {
		t.Fatal(err)
	}
	Setup(config.DefaultConfig(), log)
	for i, arg := range os.Args {
		if arg == "--" {
			Cmd.SetArgs(os.Args[i+1:])
			break
		}
	}
	if err := Cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func TestStoreCommandCLIFlags(t *testing.T) {
	for _, flags := range [][]string{{"--force"}, {"--cmd", "-f"}, {"-cf"}, {"-fc"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			source, destination := setupCommandStore(t)
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, []byte("original command\n"), 0644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-test.run=^TestStoreCommandCLIProcess$", "--", source}, flags...)
			child := exec.Command(os.Args[0], args...)
			child.Env = append(os.Environ(), "BOILER_STORE_CLI_TEST=1", "BOILER_STORE_TEST_HOME="+filepath.Dir(source))
			output, err := child.CombinedOutput()
			if len(flags) == 1 && flags[0] == "--force" {
				exitErr, ok := err.(*exec.ExitError)
				if !ok || exitErr.ExitCode() != 1 || !strings.Contains(string(output), "--force requires --command") {
					t.Fatalf("expected exit 1 and flag restriction, got %v: %s", err, output)
				}
				assertCommandContent(t, destination, "original command\n")
			} else {
				if err != nil {
					t.Fatalf("CLI failed: %v: %s", err, output)
				}
				assertCommandContent(t, destination, "new command\n")
			}
		})
	}
}
