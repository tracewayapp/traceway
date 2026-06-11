package clientcontrollers

import (
	"encoding/json"
	"testing"
)

func TestComputeExceptionHash_Java(t *testing.T) {
	// Two occurrences of the same exception with different user IDs in the
	// message must produce the same hash (message is stripped).
	stackA := "org.springframework.dao.EmptyResultDataAccessException: Incorrect result size: expected 1, actual 0\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)\n\t... 52 more"
	stackB := "org.springframework.dao.EmptyResultDataAccessException: No user found for id 99999\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)\n\t... 52 more"
	if ComputeExceptionHash(stackA, false) != ComputeExceptionHash(stackB, false) {
		t.Error("same exception with different messages should have the same hash")
	}

	// Line number changes must not change the hash.
	stackC := "java.lang.NullPointerException: Cannot invoke method\n\tat com.example.Service.run(Service.java:10)\n\tat com.example.Main.main(Main.java:5)"
	stackD := "java.lang.NullPointerException: Cannot invoke method\n\tat com.example.Service.run(Service.java:99)\n\tat com.example.Main.main(Main.java:42)"
	if ComputeExceptionHash(stackC, false) != ComputeExceptionHash(stackD, false) {
		t.Error("same exception at different line numbers should have the same hash")
	}

	// Different ellipsis counts must not change the hash.
	stackE := "java.io.IOException: timeout\n\tat com.example.Client.send(Client.java:20)\n\t... 10 more"
	stackF := "java.io.IOException: timeout\n\tat com.example.Client.send(Client.java:20)\n\t... 73 more"
	if ComputeExceptionHash(stackE, false) != ComputeExceptionHash(stackF, false) {
		t.Error("same exception with different ellipsis counts should have the same hash")
	}

	// Caused-by chains with different messages must not change the hash.
	stackG := "org.springframework.web.client.RestClientException: error\n\tat com.example.Api.call(Api.java:15)\nCaused by: java.net.SocketTimeoutException: connect timed out\n\tat java.net.Socket.connect(Socket.java:633)"
	stackH := "org.springframework.web.client.RestClientException: error\n\tat com.example.Api.call(Api.java:15)\nCaused by: java.net.SocketTimeoutException: Read timed out after 30000ms\n\tat java.net.Socket.connect(Socket.java:633)"
	if ComputeExceptionHash(stackG, false) != ComputeExceptionHash(stackH, false) {
		t.Error("same caused-by chain with different messages should have the same hash")
	}

	// Kotlin stack frames (.kt extension) must have line numbers stripped too.
	stackI := "java.lang.IllegalStateException: bad state\n\tat com.example.Service.process(Service.kt:55)"
	stackJ := "java.lang.IllegalStateException: bad state\n\tat com.example.Service.process(Service.kt:88)"
	if ComputeExceptionHash(stackI, false) != ComputeExceptionHash(stackJ, false) {
		t.Error("same Kotlin exception at different line numbers should have the same hash")
	}

	// Structurally different exceptions must NOT have the same hash.
	stackK := "java.lang.NullPointerException: npe\n\tat com.example.Foo.bar(Foo.java:1)"
	stackL := "java.lang.IllegalArgumentException: iae\n\tat com.example.Foo.bar(Foo.java:1)"
	if ComputeExceptionHash(stackK, false) == ComputeExceptionHash(stackL, false) {
		t.Error("different exception types should have different hashes")
	}
}

func TestComputeExceptionHash_JsFunctionNames(t *testing.T) {
	stackA := "Error: Test error\nanonymous()\n    ../src/app.js:3:30\nbar()\n    ../src/bar.js:4:3"
	stackB := "Error: Test error\nbuttonCallback()\n    ../src/app.js:3:30\nmodule.exports()\n    ../src/bar.js:4:3"
	if ComputeExceptionHash(stackA, false) != ComputeExceptionHash(stackB, false) {
		t.Error("same locations with different resolved function names should have the same hash")
	}

	stackC := "Error: Test error\nanonymous()\n    ../src/app.js:3:30"
	stackD := "Error: Test error\nanonymous()\n    ../src/other.js:7:12"
	if ComputeExceptionHash(stackC, false) == ComputeExceptionHash(stackD, false) {
		t.Error("different locations should have different hashes")
	}

	stackE := "Error: Test error\nanonymous()\n    bundle.min.js:1:48211"
	stackF := "Error: Test error\nanonymous()\n    bundle.min.js:1:91567"
	if ComputeExceptionHash(stackE, false) == ComputeExceptionHash(stackF, false) {
		t.Error("different columns in minified frames should have different hashes")
	}

	indentedA := "Error: Test error\n    anonymous()\n    ../src/app.js:3:30"
	indentedB := "Error: Test error\n    buttonCallback()\n    ../src/app.js:3:30"
	if ComputeExceptionHash(indentedA, false) != ComputeExceptionHash(indentedB, false) {
		t.Error("indented name lines should also be collapsed")
	}
}

func TestComputeExceptionHash_JsFnCollapseDoesNotTouchGo(t *testing.T) {
	goStackA := "runtime error: index out of range\nmain.main()\n\t/app/main.go:10 +0x20"
	goStackB := "runtime error: index out of range\nmain.other()\n\t/app/main.go:10 +0x20"
	if ComputeExceptionHash(goStackA, false) == ComputeExceptionHash(goStackB, false) {
		t.Error("Go function-name lines must not be collapsed: tab-indented file lines are not JS frames")
	}
}

func TestIsEmptyRaw(t *testing.T) {
	cases := []struct {
		name string
		in   json.RawMessage
		want bool
	}{
		{"nil", nil, true},
		{"empty bytes", json.RawMessage(""), true},
		{"whitespace null", json.RawMessage(" null "), true},
		{"plain null", json.RawMessage("null"), true},
		{"empty array", json.RawMessage("[]"), true},
		{"empty object", json.RawMessage("{}"), true},
		{"non-empty array", json.RawMessage("[1]"), false},
		{"non-empty object", json.RawMessage(`{"a":1}`), false},
		{"string value", json.RawMessage(`"x"`), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isEmptyRaw(c.in); got != c.want {
				t.Fatalf("isEmptyRaw(%q) = %v, want %v", string(c.in), got, c.want)
			}
		})
	}
}

func TestComputeExceptionHash_DropsColumnsBeyondLineOne(t *testing.T) {
	resolvedChrome := "TypeError: boom\nassertValid()\n    ../../src/pricing.ts:23:11\napplyDiscount()\n    ../../src/pricing.ts:17:3"
	resolvedWebkit := "TypeError: boom\nassertValid()\n    ../../src/pricing.ts:23:15\napplyDiscount()\n    ../../src/pricing.ts:17:9"
	if ComputeExceptionHash(resolvedChrome, false) != ComputeExceptionHash(resolvedWebkit, false) {
		t.Error("resolved frames with engine-specific columns must hash identically")
	}

	minifiedA := "TypeError: boom\nfn()\n    app.min.js:1:730"
	minifiedB := "TypeError: boom\nfn()\n    app.min.js:1:999"
	if ComputeExceptionHash(minifiedA, false) == ComputeExceptionHash(minifiedB, false) {
		t.Error("line-1 minified frames must keep the column as disambiguator")
	}
}

func TestComputeExceptionHash_GoTraceUnaffectedByJsNormalizers(t *testing.T) {
	goTrace := "runtime error: invalid memory address or nil pointer dereference\nmain.handleOrder(0x0)\n\t/srv/app/internal/orders/handler.go:42 +0x1b\nmain.main()\n\t/srv/app/main.go:18 +0x2f\ngoroutine 17 [running]:"
	if urlOriginRe.MatchString(goTrace) {
		t.Error("urlOriginRe must not match Go traces")
	}
	if laterLineColRe.MatchString(goTrace) {
		t.Error("laterLineColRe must not match Go traces, they have no :line:col suffix")
	}
	if jsFuncLineRe.MatchString(goTrace) {
		t.Error("jsFuncLineRe must not match tab-indented Go traces")
	}
	if ComputeExceptionHash(goTrace, false) != "5fa84c6186649978" {
		t.Errorf("Go trace hash drifted: %s (update only if grouping intentionally changed)", ComputeExceptionHash(goTrace, false))
	}
}
