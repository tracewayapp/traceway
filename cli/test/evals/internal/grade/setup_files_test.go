package grade

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// fakeToken stands in for a project token in the setup grader tests.
const fakeToken = "0123456789abcdef0123456789abcdef"

type tokenOnly struct{}

func (tokenOnly) Get(context.Context, string, map[string]string) (int, []byte, error) {
	return 0, nil, nil
}
func (tokenOnly) Post(context.Context, string, map[string]string, any) (int, []byte, error) {
	return 0, nil, nil
}
func (tokenOnly) ProjectID(string) string { return "" }
func (tokenOnly) PrimaryProject() string  { return "" }
func (tokenOnly) OrgID() int              { return 0 }
func (tokenOnly) Tokens() []string        { return []string{fakeToken} }

// fileEdit changes one workspace file: with old empty, new is the whole
// file (empty new with empty old deletes it); otherwise old is replaced
// once.
type fileEdit struct{ path, old, new string }

type graderVariant struct {
	name  string
	edits []fileEdit
	pass  bool
	// note: the failure carries the assertion's note (the 2.0 start).
	note bool
}

// setupGraderCases holds, per setup case, what the oracle's shell steps
// would have left (installed packages, env files an apply step wrote,
// gitignored local secrets), and variants of the result: equivalent shapes
// that must pass, and the 2.0 start and known mistakes that must fail.
var setupGraderCases = map[string]struct {
	afterOracle []fileEdit
	variants    []graderVariant
}{
	"setup-react-browser": {
		afterOracle: []fileEdit{
			{path: "package.json", old: `"react": "^19.1.0",`, new: `"@tracewayapp/react": "^2.1.0",
    "react": "^19.1.0",`},
			{path: "package.json", old: `"@vitejs/plugin-react": "^4.7.0",`, new: `"@tracewayapp/bundler-plugin": "^2.1.0",
    "@tracewayapp/sourcemap-upload": "^2.1.0",
    "@vitejs/plugin-react": "^4.7.0",`},
		},
		variants: []graderVariant{
			{name: "props from an object spread in", pass: true, edits: []fileEdit{{path: "src/main.jsx", new: `import { createRoot } from "react-dom/client";
import { TracewayProvider } from "@tracewayapp/react";
import App from "./App.jsx";

const traceway = { token: import.meta.env.VITE_TRACEWAY_TOKEN, baseUrl: import.meta.env.VITE_TRACEWAY_URL };

createRoot(document.getElementById("root")).render(<TracewayProvider {...traceway}><App /></TracewayProvider>);
`}}},
			{name: "props from variables", pass: true, edits: []fileEdit{{path: "src/main.jsx", new: `import { createRoot } from "react-dom/client";
import { TracewayProvider } from "@tracewayapp/react";
import App from "./App.jsx";

const { VITE_TRACEWAY_TOKEN: token, VITE_TRACEWAY_URL: url } = import.meta.env;

createRoot(document.getElementById("root")).render(
  <TracewayProvider
    token={token}
    baseUrl={url}
  >
    <App />
  </TracewayProvider>
);
`}}},
			{name: "the 2.0 connection string", note: true, edits: []fileEdit{{path: "src/main.jsx",
				old: `<TracewayProvider token={import.meta.env.VITE_TRACEWAY_TOKEN} baseUrl={import.meta.env.VITE_TRACEWAY_URL}>`,
				new: "<TracewayProvider connectionString={`${import.meta.env.VITE_TRACEWAY_TOKEN}@${import.meta.env.VITE_TRACEWAY_URL}/api/report`}>"}}},
			{name: "a 1.x pin", edits: []fileEdit{{path: "package.json", old: `"@tracewayapp/react": "^2.1.0"`, new: `"@tracewayapp/react": "^1.2.1"`}}},
			{name: "a 2.0 pin", edits: []fileEdit{{path: "package.json", old: `"@tracewayapp/react": "^2.1.0"`, new: `"@tracewayapp/react": "^2.0.3"`}}},
			{name: "no baseUrl for the self-hosted server", edits: []fileEdit{{path: "src/main.jsx", old: ` baseUrl={import.meta.env.VITE_TRACEWAY_URL}`, new: ``}}},
		},
	},
	"setup-monorepo-plan": {
		afterOracle: []fileEdit{
			{path: "web/package.json", old: `"dependencies": {`, new: `"dependencies": {
    "@tracewayapp/react": "^2.1.0",`},
			{path: "web/.env.local", new: "VITE_TRACEWAY_TOKEN=" + fakeToken + "\nVITE_TRACEWAY_URL=http://127.0.0.1:18082\n"},
			{path: "api/.env", new: "TRACEWAY_API_TOKEN=" + fakeToken + "\nTRACEWAY_URL=http://127.0.0.1:18082\n"},
			{path: "api/go.mod", old: "\ngo ", new: "\nrequire go.opentelemetry.io/otel v1.47.0\n\ngo "},
		},
		variants: []graderVariant{
			{name: "a plan with the connectionString envFormat", note: true, edits: []fileEdit{
				{path: "web/.env.local", new: "VITE_TRACEWAY_TOKEN=" + fakeToken + "@http://127.0.0.1:18082\n"}}},
			{name: "no server URL next to the token", edits: []fileEdit{
				{path: "web/.env.local", new: "VITE_TRACEWAY_TOKEN=" + fakeToken + "\n"}}},
		},
	},
	"setup-flutter": {
		variants: []graderVariant{
			{name: "constants and --dart-define-from-file", pass: true, edits: []fileEdit{
				{path: "lib/main.dart", old: `void main() {`, new: `const _token = String.fromEnvironment('TRACEWAY_TOKEN');
const _url = String.fromEnvironment('TRACEWAY_URL');

void main() {`},
				{path: "lib/main.dart", old: `    token: const String.fromEnvironment('TRACEWAY_TOKEN'),
    baseUrl: const String.fromEnvironment('TRACEWAY_URL'),`, new: `    token: _token,
    baseUrl: _url,`},
				{path: ".github/workflows/release.yml", new: `on: push
jobs:
  android:
    runs-on: ubuntu-latest
    steps:
      - run: printf 'TRACEWAY_TOKEN=%s\nTRACEWAY_URL=%s\n' "${{ secrets.TRACEWAY_TOKEN }}" "${{ vars.TRACEWAY_URL }}" > .env
      - run: |
          flutter build appbundle --release \
            --dart-define-from-file=.env
  ios:
    runs-on: macos-15
    steps:
      - run: printf 'TRACEWAY_TOKEN=%s\nTRACEWAY_URL=%s\n' "${{ secrets.TRACEWAY_TOKEN }}" "${{ vars.TRACEWAY_URL }}" > .env
      - run: flutter build ipa --release --dart-define-from-file=.env
`},
				{path: ".env", new: "TRACEWAY_TOKEN=" + fakeToken + "\n"}}},
			{name: "the 2.0 connection string", note: true, edits: []fileEdit{{path: "lib/main.dart",
				old: `    token: const String.fromEnvironment('TRACEWAY_TOKEN'),
    baseUrl: const String.fromEnvironment('TRACEWAY_URL'),`,
				new: `    connectionString: const String.fromEnvironment('TRACEWAY_CONNECTION_STRING'),`}}},
			{name: "video mode without the add-on", edits: []fileEdit{{path: "lib/main.dart", old: "  TracewayVideo.register();\n", new: ""}}},
			{name: "the add-on registered after run", edits: []fileEdit{
				{path: "lib/main.dart", old: "  TracewayVideo.register();\n", new: ""},
				{path: "lib/main.dart", old: "    child: const ShopApp(),\n  );\n", new: "    child: const ShopApp(),\n  );\n  TracewayVideo.register();\n"}}},
			{name: "a 2.0 pin", edits: []fileEdit{{path: "pubspec.yaml", old: "traceway: ^2.1.0", new: "traceway: ^2.0.0"}}},
			{name: "no navigator observer", edits: []fileEdit{{path: "lib/main.dart", old: "      navigatorObservers: [Traceway.navigatorObserver],\n", new: ""}}},
			{name: "only one store build gets the token", edits: []fileEdit{{path: ".github/workflows/release.yml",
				old: `flutter build ios --release --no-codesign --dart-define=TRACEWAY_TOKEN="$TRACEWAY_TOKEN" --dart-define=TRACEWAY_URL="$TRACEWAY_URL"`,
				new: `flutter build ios --release --no-codesign`}}},
			{name: "the token in a committed launch config", edits: []fileEdit{{path: ".vscode/launch.json",
				new: `{"configurations": [{"name": "shop", "request": "launch", "type": "dart", "args": ["--dart-define=TRACEWAY_TOKEN=` + fakeToken + `"]}]}`}}},
		},
	},
	"setup-android": {
		afterOracle: []fileEdit{{path: "local.properties", new: "TRACEWAY_TOKEN=" + fakeToken + "\nTRACEWAY_URL=http://127.0.0.1:18082\n"}},
		variants: []graderVariant{
			{name: "named arguments, a local val and autoUpload", pass: true, edits: []fileEdit{
				{path: "app/src/main/java/com/example/pinboard/PinboardApp.kt", new: `package com.example.pinboard

import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class PinboardApp : Application() {
    override fun onCreate() {
        super.onCreate()
        val tracewayToken = BuildConfig.TRACEWAY_TOKEN
        Traceway.init(
            application = this,
            options = TracewayOptions(token = tracewayToken, baseUrl = BuildConfig.TRACEWAY_URL.ifBlank { null }, screenCapture = true),
        )
    }
}
`},
				{path: "app/build.gradle.kts", old: "    url = localOrEnv(\"TRACEWAY_URL\")\n", new: "    url = localOrEnv(\"TRACEWAY_URL\")\n    autoUpload = true\n"},
				{path: ".github/workflows/release.yml", old: "gradle :app:bundleRelease :app:uploadReleaseTracewaySymbols", new: "gradle :app:bundleRelease"}}},
			{name: "the 2.0 connection string", note: true, edits: []fileEdit{{path: "app/src/main/java/com/example/pinboard/PinboardApp.kt",
				old: `        Traceway.init(
            this,
            TracewayOptions(
                token = BuildConfig.TRACEWAY_TOKEN,
                baseUrl = BuildConfig.TRACEWAY_URL,
                screenCapture = true,
            ),
        )`,
				new: `        Traceway.init(this, BuildConfig.TRACEWAY_DSN, TracewayOptions(screenCapture = true))`}}},
			{name: "an application class the manifest does not name", edits: []fileEdit{{path: "app/src/main/AndroidManifest.xml", old: "        android:name=\".PinboardApp\"\n", new: ""}}},
			{name: "minSdk raised instead of desugaring", edits: []fileEdit{
				{path: "app/build.gradle.kts", old: "minSdk = 24", new: "minSdk = 26"},
				{path: "app/build.gradle.kts", old: "        isCoreLibraryDesugaringEnabled = true\n", new: ""}}},
			{name: "the plugin's url left commented out", edits: []fileEdit{{path: "app/build.gradle.kts", old: "    url = localOrEnv(\"TRACEWAY_URL\")\n", new: "    // url = \"https://traceway.mycorp.com\"\n"}}},
			{name: "a 2.0 SDK", edits: []fileEdit{{path: "app/build.gradle.kts", old: "com.tracewayapp:traceway:2.1.0", new: "com.tracewayapp:traceway:2.0.0-beta.1"}}},
			{name: "the token in the committed gradle.properties", edits: []fileEdit{{path: "gradle.properties", old: "android.useAndroidX=true\n", new: "android.useAndroidX=true\nTRACEWAY_TOKEN=" + fakeToken + "\n"}}},
		},
	},
	"setup-ios": {
		afterOracle: []fileEdit{{path: "Config/Secrets.xcconfig", new: "TRACEWAY_TOKEN = " + fakeToken + "\nTRACEWAY_URL = http:/$()/127.0.0.1:18082\n"}},
		variants: []graderVariant{
			{name: "an app delegate, infoDictionary and an untracked Traceway.xcconfig", pass: true, edits: []fileEdit{
				{path: "Shop/ShopApp.swift", new: `import SwiftUI
import Traceway

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        let info = Bundle.main.infoDictionary ?? [:]
        Traceway.initialize(
            token: info["TRACEWAY_TOKEN"] as? String ?? "",
            baseUrl: info["TRACEWAY_URL"] as? String ?? "",
            options: TracewayOptions(version: "3.12.0", screenCapture: true)
        )
        return true
    }
}

@main
struct ShopApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @State private var cart = Cart()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(cart)
        }
    }
}
`},
				{path: "Shop/Info.plist", old: "<key>TracewayToken</key>", new: "<key>TRACEWAY_TOKEN</key>"},
				{path: ".github/workflows/release.yml", old: `printf 'TRACEWAY_TOKEN = %s\n' "$TRACEWAY_TOKEN"`, new: `printf 'TRACEWAY_TOKEN = %s\n' "$TRACEWAY_TOKEN" > Config/Traceway.xcconfig`}}},
			{name: "the 2.0 connection string", note: true, edits: []fileEdit{{path: "Shop/ShopApp.swift",
				old: `        Traceway.initialize(
            token: Bundle.main.object(forInfoDictionaryKey: "TracewayToken") as? String ?? "",
            baseUrl: Bundle.main.object(forInfoDictionaryKey: "TracewayURL") as? String ?? "",`,
				new: `        Traceway.initialize(
            connectionString: Bundle.main.object(forInfoDictionaryKey: "TracewayToken") as? String,`}}},
			{name: "the deprecated mask name", note: true, edits: []fileEdit{{path: "Shop/Checkout/CheckoutView.swift", old: ".tracewayMask(.blank())", new: ".tracewayPrivacyMask(.blank())"}}},
			{name: "the accessibility identifier SwiftUI cannot pass on", edits: []fileEdit{{path: "Shop/Checkout/CheckoutView.swift", old: ".tracewayMask(.blank())", new: `.accessibilityIdentifier("traceway-mask")`}}},
			{name: "started from a view's task", edits: []fileEdit{{path: "Shop/ShopApp.swift", old: "    init() {\n", new: "    func start() {\n"}}},
			{name: "a package the target does not link", edits: []fileEdit{{path: "Shop.xcodeproj/project.pbxproj",
				old: "\t\t\tpackageProductDependencies = (\n\t\t\t\t8E4C1F152D7A9B0C3E5F6A1B /* Traceway */,\n\t\t\t);",
				new: "\t\t\tpackageProductDependencies = (\n\t\t\t);"}}},
			{name: "a 2.0 requirement", edits: []fileEdit{{path: "Shop.xcodeproj/project.pbxproj", old: "minimumVersion = 2.1.0;", new: "minimumVersion = 2.0.0-beta.1;"}}},
			{name: "the token in the committed xcconfig", edits: []fileEdit{{path: "Config/Shop.xcconfig", old: "API_BASE_URL", new: "TRACEWAY_TOKEN = " + fakeToken + "\nAPI_BASE_URL"}}},
		},
	},
}

// TestSetupFileGraders plays the file assertions of every setup case
// (file kinds, no_token_literal) against workspaces made from its
// template: the template as it is must fail, the oracle's result must
// pass, and so must each equivalent shape; the 2.0 start and the known
// mistakes must fail, the 2.0 start with the assertion's note.
func TestSetupFileGraders(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git: no_token_literal reads the files git would commit")
	}
	root := env.EvalsRoot()
	cases, err := catalog.LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, c := range cases {
		if c.Suite != "setup" {
			continue
		}
		spec, ok := setupGraderCases[c.ID]
		if !ok {
			continue
		}
		seen++
		t.Run(c.ID, func(t *testing.T) {
			fileCase := *c
			fileCase.Expect = catalog.Expect{}
			notes := 0
			for _, a := range c.Expect.State {
				if a.Kind == "file" || a.Kind == "no_token_literal" {
					fileCase.Expect.State = append(fileCase.Expect.State, a)
					if a.Note != "" {
						notes++
					}
				}
			}
			template := filepath.Join(root, "testdata", "setup-templates", c.Workspace.Repo)
			build := func(t *testing.T, edits ...[]fileEdit) string {
				ws := t.TempDir()
				copyTemplate(t, template, ws)
				applyOracle(t, root, ws, c.Oracle)
				for _, list := range edits {
					applyEdits(t, ws, list)
				}
				if out, err := exec.Command("git", "-C", ws, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v: %s", err, out)
				}
				return ws
			}
			grade := func(ws string) *Verdict {
				return State(context.Background(), &Input{Case: &fileCase, Workspace: ws, State: tokenOnly{}})
			}

			pristine := t.TempDir()
			copyTemplate(t, template, pristine)
			_ = exec.Command("git", "-C", pristine, "init", "-q").Run()
			if v := grade(pristine); v.Pass {
				t.Error("the untouched template passes the file assertions")
			}
			if v := grade(build(t, spec.afterOracle)); !v.Pass {
				t.Fatalf("the oracle's result fails:\n%s", strings.Join(v.Failures, "\n"))
			}
			for _, vr := range spec.variants {
				v := grade(build(t, spec.afterOracle, vr.edits))
				switch {
				case vr.pass && !v.Pass:
					t.Errorf("%s: fails, want pass:\n%s", vr.name, strings.Join(v.Failures, "\n"))
				case !vr.pass && v.Pass:
					t.Errorf("%s: passes, want a failure", vr.name)
				case vr.note && notes > 0 && !anyNote(v, fileCase.Expect.State):
					t.Errorf("%s: the failure does not carry the note:\n%s", vr.name, strings.Join(v.Failures, "\n"))
				}
			}
		})
	}
	if seen < 5 {
		t.Fatalf("only %d setup cases have grader fixtures", seen)
	}
}

func anyNote(v *Verdict, asserts []catalog.StateAssert) bool {
	for _, f := range v.Failures {
		for _, a := range asserts {
			if a.Note != "" && strings.Contains(f, a.Note) {
				return true
			}
		}
	}
	return false
}

func copyTemplate(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// applyOracle replays the oracle's file steps; its shell steps are the
// afterOracle edits of setupGraderCases.
func applyOracle(t *testing.T, root, ws string, steps []catalog.Step) {
	t.Helper()
	for _, s := range steps {
		switch {
		case s.WriteFile != nil:
			content := s.WriteFile.Content
			if s.WriteFile.From != "" {
				raw, err := os.ReadFile(filepath.Join(root, "testdata", s.WriteFile.From))
				if err != nil {
					t.Fatal(err)
				}
				content = string(raw)
			}
			applyEdits(t, ws, []fileEdit{{path: s.WriteFile.Path, new: content}})
		case s.EditFile != nil:
			applyEdits(t, ws, []fileEdit{{path: s.EditFile.Path, old: s.EditFile.Old, new: s.EditFile.New}})
		}
	}
}

func applyEdits(t *testing.T, ws string, edits []fileEdit) {
	t.Helper()
	for _, e := range edits {
		p := filepath.Join(ws, filepath.FromSlash(e.path))
		if e.old == "" {
			if e.new == "" {
				_ = os.Remove(p)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(e.new), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(raw), e.old); n != 1 {
			t.Fatalf("%s: %q occurs %d times, want once", e.path, e.old, n)
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(raw), e.old, e.new, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
