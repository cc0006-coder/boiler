package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
