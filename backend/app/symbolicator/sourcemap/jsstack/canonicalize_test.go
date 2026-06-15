package jsstack

import (
	"strings"
	"testing"
)

func TestCanonicalizeChrome(t *testing.T) {
	input := strings.Join([]string{
		"TypeError: Cannot read properties of undefined (reading 'name')",
		"    at renderUser (https://app.example.com/assets/app.min.js:1:13337)",
		"    at https://app.example.com/assets/app.min.js:1:24601",
		"    at async loadPage (https://app.example.com/assets/app.min.js:2:99)",
		"    at new Widget (https://app.example.com/assets/vendor.min.js:1:42)",
		"    at Object.run [as start] (https://app.example.com/assets/app.min.js:1:7)",
		"    at Array.forEach (<anonymous>)",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected chrome trace to be detected")
	}
	want := strings.Join([]string{
		"TypeError: Cannot read properties of undefined (reading 'name')",
		"renderUser()",
		"    https://app.example.com/assets/app.min.js:1:13337",
		"    https://app.example.com/assets/app.min.js:1:24601",
		"loadPage()",
		"    https://app.example.com/assets/app.min.js:2:99",
		"Widget()",
		"    https://app.example.com/assets/vendor.min.js:1:42",
		"Object.run()",
		"    https://app.example.com/assets/app.min.js:1:7",
		"    Array.forEach [native code]",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizeNodeAndBun(t *testing.T) {
	input := strings.Join([]string{
		"Error: connect ECONNREFUSED",
		"    at TCPConnectWrap.afterConnect [as oncomplete] (node:net:1300:16)",
		"    at Module._compile (node:internal/modules/cjs/loader:1234:14)",
		"    at /srv/app/dist/server.js:10:5",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected node trace to be detected")
	}
	want := strings.Join([]string{
		"Error: connect ECONNREFUSED",
		"TCPConnectWrap.afterConnect()",
		"    node:net:1300:16",
		"Module._compile()",
		"    node:internal/modules/cjs/loader:1234:14",
		"    /srv/app/dist/server.js:10:5",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizeChromeEval(t *testing.T) {
	input := strings.Join([]string{
		"Error: boom",
		"    at eval (eval at run (https://x.test/app.min.js:1:100), <anonymous>:5:9)",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected eval trace to be detected")
	}
	want := "Error: boom\neval()\n    https://x.test/app.min.js:1:100"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizeFirefox(t *testing.T) {
	input := strings.Join([]string{
		"TypeError: user is undefined",
		"renderUser@https://app.example.com/assets/app.min.js:1:13337",
		"@https://app.example.com/assets/app.min.js:1:24601",
		"outer@https://app.example.com/assets/app.min.js line 2 > eval:1:1",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected firefox trace to be detected")
	}
	want := strings.Join([]string{
		"TypeError: user is undefined",
		"renderUser()",
		"    https://app.example.com/assets/app.min.js:1:13337",
		"    https://app.example.com/assets/app.min.js:1:24601",
		"outer()",
		"    https://app.example.com/assets/app.min.js:2:1",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizeFirefoxNestedEvalMarkers(t *testing.T) {
	input := strings.Join([]string{
		"Error: deep",
		"f@https://x.test/app.min.js line 3 > eval line 1 > eval:1:9",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected firefox eval trace to be detected")
	}
	want := strings.Join([]string{
		"Error: deep",
		"f()",
		"    https://x.test/app.min.js:3:1",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizeSafari(t *testing.T) {
	input := strings.Join([]string{
		"renderUser@https://app.example.com/assets/app.min.js:1:13337",
		"global code@https://app.example.com/assets/app.min.js:1:24601",
		"promiseReactionJob@[native code]",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected safari trace to be detected")
	}
	want := strings.Join([]string{
		"renderUser()",
		"    https://app.example.com/assets/app.min.js:1:13337",
		"global code()",
		"    https://app.example.com/assets/app.min.js:1:24601",
		"    promiseReactionJob [native code]",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCanonicalizePassthrough(t *testing.T) {
	cases := map[string]string{
		"canonical": "Error: boom\nanonymous()\n    minified.js:1:11",
		"go":        "runtime error: invalid memory address\nmain.handler(0x0)\n\t/srv/app/main.go:42 +0x1b",
		"message":   "Script error.",
		"email":     "Error: mail user@example.com bounced",
		"empty":     "",
	}
	for name, input := range cases {
		got, ok := Canonicalize(input)
		if ok {
			t.Errorf("%s: expected no detection", name)
		}
		if got != input {
			t.Errorf("%s: trace must pass through unchanged, got %q", name, got)
		}
	}
}

func TestCanonicalizeFirefoxDropsSyntheticFrames(t *testing.T) {
	input := strings.Join([]string{
		"TypeError: x is undefined",
		"assertValid@https://x.test/app.min.js:1:730",
		"handleCheckout@https://x.test/app.min.js:1:1383",
		"handleEvent*@https://x.test/app.min.js:1:2067",
		"async*loadRate@https://x.test/app.min.js:1:2223",
	}, "\n")

	got, ok := Canonicalize(input)
	if !ok {
		t.Fatal("expected firefox trace to be detected")
	}
	want := strings.Join([]string{
		"TypeError: x is undefined",
		"assertValid()",
		"    https://x.test/app.min.js:1:730",
		"handleCheckout()",
		"    https://x.test/app.min.js:1:1383",
		"loadRate()",
		"    https://x.test/app.min.js:1:2223",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
