package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

func androidFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", "symbolicator", "android", "fixtures"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestResolveAndroidStackTrace(t *testing.T) {
	prev := storage.Store
	defer func() { storage.Store = prev }()
	store, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storage.Store = store

	ctx := context.Background()
	projectId := uuid.New()
	const proguardUuid = "11112222-3333-4444-5555-666677778888"

	mapping := androidFixture(t, "r8", "mapping.txt")
	if err := store.Write(ctx, AndroidMappingKey(projectId, proguardUuid), mapping); err != nil {
		t.Fatal(err)
	}

	t.Run("r8", func(t *testing.T) {
		obf := string(androidFixture(t, "r8", "obfuscated.txt"))
		got := ResolveAndroidStackTrace(ctx, projectId, obf, proguardUuid)
		if strings.Contains(got, "a.a.a(") {
			t.Errorf("output still obfuscated:\n%s", got)
		}
		for _, want := range []string{"com.example.demo.Checkout.chargeCard", "Checkout.java", "com.example.demo.Main.main"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in retraced output:\n%s", want, got)
			}
		}
	})

	t.Run("no_uuid_skips_r8", func(t *testing.T) {
		obf := string(androidFixture(t, "r8", "obfuscated.txt"))
		got := ResolveAndroidStackTrace(ctx, projectId, obf, "")
		if !strings.Contains(got, "a.a.a(") {
			t.Errorf("expected trace to stay obfuscated without a proguard uuid:\n%s", got)
		}
	})

	t.Run("non_android_passthrough", func(t *testing.T) {
		raw := "TypeError: boom\n    at foo (app.js:1:2)"
		if got := ResolveAndroidStackTrace(ctx, projectId, raw, proguardUuid); got != raw {
			t.Errorf("non-android trace mutated: %q", got)
		}
	})
}

func TestResolveAndroidR8MemoryAndDiskAgree(t *testing.T) {
	prevStore := storage.Store
	defer func() { storage.Store = prevStore }()
	store, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storage.Store = store

	ctx := context.Background()
	projectId := uuid.New()
	const proguardUuid = "22223333-4444-5555-6666-777788889999"
	if err := store.Write(ctx, AndroidMappingKey(projectId, proguardUuid), androidFixture(t, "r8", "mapping.txt")); err != nil {
		t.Fatal(err)
	}
	obf := string(androidFixture(t, "r8", "obfuscated.txt"))

	if symbolicatorOnDisk() {
		t.Fatal("expected an in-memory shared cache by default")
	}
	mem := ResolveAndroidStackTrace(ctx, projectId, obf, proguardUuid)
	if !strings.Contains(mem, "com.example.demo.Checkout.chargeCard") {
		t.Fatalf("memory-mode retrace missing expected frame:\n%s", mem)
	}

	prevCache := sharedCache
	defer func() { sharedCache = prevCache }()
	if err := EnableSymbolicatorDiskCache(t.TempDir(), 64<<20); err != nil {
		t.Fatal(err)
	}
	if !symbolicatorOnDisk() {
		t.Fatal("expected a disk-backed shared cache after EnableSymbolicatorDiskCache")
	}
	disk := ResolveAndroidStackTrace(ctx, projectId, obf, proguardUuid)

	if mem != disk {
		t.Errorf("memory and disk retrace differ:\nmem:  %q\ndisk: %q", mem, disk)
	}
}
