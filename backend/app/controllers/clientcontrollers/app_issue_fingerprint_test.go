package clientcontrollers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// appIssueTrace is a fixture as the issue stores it: the SDK's exception.type, then its message and frames.
func appIssueTrace(t *testing.T, typ, fixture string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "app_issues", fixture))
	if err != nil {
		t.Fatal(err)
	}
	return typ + ": " + strings.TrimRight(string(b), "\n")
}

func TestAppIssueTypesGroupByTypeAndTopInAppFrames(t *testing.T) {
	cases := []struct {
		name, typ, a, b string
	}{
		// Two installs of one build on two Android versions: random install directories, other libart and libc
		// builds (BuildIds, program counters, symbol offsets), a different fault address.
		{"native crash on two devices", "NativeCrash", "native_crash_pixel7.txt", "native_crash_galaxy.txt"},
		// The same blocked call site: other line numbers in the framework, another lock address, waiting time and
		// thread, another input event.
		{"ANR on two devices", "ApplicationNotResponding", "anr_blocked_main.txt", "anr_blocked_main_again.txt"},
		// Two samples of one hang: blocked for a different time, one frame deeper in the kernel, on another iOS build.
		{"iOS hang sampled twice", "AppHang", "ios_hang_sample_1.txt", "ios_hang_sample_2.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := appIssueTrace(t, tc.typ, tc.a), appIssueTrace(t, tc.typ, tc.b)
			if ha, hb := ComputeExceptionHash(a, false), ComputeExceptionHash(b, false); ha != hb {
				t.Errorf("two occurrences of one %s are two issues:\n%s\n%s", tc.typ, appIssueFingerprint(tc.typ, a), appIssueFingerprint(tc.typ, b))
			}
		})
	}
}

func TestAppIssueFingerprintKeepsTheAppsFrames(t *testing.T) {
	cases := []struct {
		typ, fixture, want string
	}{
		{"NativeCrash", "native_crash_pixel7.txt", strings.Join([]string{"NativeCrash",
			"libpayments.so pay::Card::charge(int)",
			"libpayments.so Java_com_example_shop_Payments_nativeCharge",
			"com.example.shop.Payments.charge",
			"com.example.shop.CheckoutActivity.onPay"}, "\n")},
		// A library inside the APK, without symbols; JIT-compiled app code in a deleted memfd.
		{"NativeCrash", "native_crash_stripped.txt", strings.Join([]string{"NativeCrash",
			"libimage.so pc 11d004",
			"libimage.so pc 10a7c",
			"com.example.shop.images.Decoder.decode"}, "\n")},
		{"ApplicationNotResponding", "anr_blocked_main.txt", strings.Join([]string{"ApplicationNotResponding",
			"com.example.shop.cart.CartRepository.load",
			"com.example.shop.cart.CartViewModel.refresh",
			"com.example.shop.CheckoutActivity.onResume"}, "\n")},
		// An idle looper has no frame of the app's: its top frames are what tells it apart.
		{"ApplicationNotResponding", "anr_idle_looper.txt", strings.Join([]string{"ApplicationNotResponding",
			"libc.so __epoll_pwait",
			"libutils.so android::Looper::pollOnce(int, int*, int*, void**)",
			"libandroid_runtime.so android::android_os_MessageQueue_nativePollOnce(_JNIEnv*, _jobject*, long, int)",
			"android.os.MessageQueue.nativePollOnce",
			"android.os.MessageQueue.next"}, "\n")},
		{"AppHang", "ios_hang_sample_1.txt", "AppHang\nShopApp+0x1a2b4\nShopApp+0x19f80\nShopApp+0x8c40"},
		// MetricKit names images by a UUID with dashes.
		{"WatchdogTermination", "ios_watchdog_metrickit.txt", "WatchdogTermination\nShopApp+0x55e10\nShopApp+0x54a08"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			if got := appIssueFingerprint(tc.typ, appIssueTrace(t, tc.typ, tc.fixture)); got != tc.want {
				t.Errorf("fingerprint\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestAppIssueTypesStayApartWhereTheAppDiffers(t *testing.T) {
	crash := appIssueTrace(t, "NativeCrash", "native_crash_pixel7.txt")
	if ComputeExceptionHash(crash, false) == ComputeExceptionHash(appIssueTrace(t, "NativeCrash", "native_crash_other_site.txt"), false) {
		t.Error("native crashes in two functions of the app are one issue")
	}
	anr := appIssueTrace(t, "ApplicationNotResponding", "anr_blocked_main.txt")
	if ComputeExceptionHash(anr, false) == ComputeExceptionHash(appIssueTrace(t, "ApplicationNotResponding", "anr_idle_looper.txt"), false) {
		t.Error("an ANR in the app's code and one on an idle looper are one issue")
	}
	// The frames say where; the type says what happened there.
	hang := "AppHang: Main thread blocked for 2.4 s\n" + strings.SplitN(anr, "\n", 2)[1]
	if ComputeExceptionHash(hang, false) == ComputeExceptionHash(anr, false) {
		t.Error("a hang and an ANR at the same frames are one issue")
	}
}

func TestAppIssueWithoutFramesGroupsByItsMessage(t *testing.T) {
	a := "WatchdogTermination: Watchdog terminated the app (0x8badf00d) after 10.2 s in the foreground"
	b := "WatchdogTermination: Watchdog terminated the app (0x8badf00d) after 20.0 s in the foreground"
	c := "WatchdogTermination: Watchdog terminated the app (0x8badf00d) during launch"
	if ComputeExceptionHash(a, false) != ComputeExceptionHash(b, false) {
		t.Error("the time a watchdog waited splits one issue")
	}
	if ComputeExceptionHash(a, false) == ComputeExceptionHash(c, false) {
		t.Error("two different watchdog messages without frames are one issue")
	}
	// Below API 31 Android has only the exit's description.
	if ComputeExceptionHash("NativeCrash: crash", false) != ComputeExceptionHash("NativeCrash: crash", false) {
		t.Error("the hash is not stable")
	}
	if typ, ok := frameGroupedIssueType("NativeCrash"); !ok || typ != "NativeCrash" {
		t.Errorf("a bare type line is %q, %v", typ, ok)
	}
}

func TestOtherExceptionsKeepTheirWholeTraceHash(t *testing.T) {
	// A class that only ends in one of the names is an ordinary exception, grouped by its whole trace as before.
	for _, trace := range []string{
		"com.example.AppHang: stuck\n\tat com.example.Foo.bar(Foo.kt:1)",
		"AppHangs: stuck\n\tat com.example.Foo.bar(Foo.kt:1)",
		"java.lang.IllegalStateException: NativeCrash\n\tat com.example.Foo.bar(Foo.kt:1)",
	} {
		if _, ok := frameGroupedIssueType(trace); ok {
			t.Errorf("%q is grouped by its top frames", trace)
		}
	}
	a := "java.lang.IllegalStateException: x\n\tat com.example.Foo.bar(Foo.kt:1)\n\tat com.example.Foo.baz(Foo.kt:2)"
	b := "java.lang.IllegalStateException: x\n\tat com.example.Foo.bar(Foo.kt:1)\n\tat com.example.Other.baz(Other.kt:2)"
	if ComputeExceptionHash(a, false) == ComputeExceptionHash(b, false) {
		t.Error("an ordinary exception lost a frame of its hash")
	}
	if ComputeExceptionHash("AppHang: Main thread blocked for 3.2 s", true) == ComputeExceptionHash("AppHang: Main thread blocked for 5.0 s", true) {
		t.Error("a message is grouped like an issue")
	}
}

func TestSplitNativeFrame(t *testing.T) {
	cases := []struct{ rest, path, symbol string }{
		{"/apex/com.android.runtime/lib64/bionic/libc.so (abort+164) (BuildId: 1c3a)", "/apex/com.android.runtime/lib64/bionic/libc.so", "abort"},
		{"/system/lib64/libutils.so (android::Looper::pollInner(int)+184) (BuildId: 8a9b)", "/system/lib64/libutils.so", "android::Looper::pollInner(int)"},
		{"/data/app/x/base.apk!libimage.so (offset 0x2c4000) (BuildId: 3e4f)", "/data/app/x/base.apk!libimage.so", ""},
		{"/memfd:jit-cache (deleted) (offset 0x2000000) (com.example.Decoder.decode+512)", "/memfd:jit-cache", "com.example.Decoder.decode"},
		{"[anon:dalvik-jit-code-cache]", "[anon:dalvik-jit-code-cache]", ""},
		{"<anonymous:7b1f4000>", "<anonymous:7b1f4000>", ""},
		{"/system/lib64/libfoo.so (unbalanced", "/system/lib64/libfoo.so (unbalanced", ""},
		{"/system/lib64/libfoo.so (a+1) (b+2)", "/system/lib64/libfoo.so (a+1)", "b"},
	}
	for _, tc := range cases {
		path, symbol := splitNativeFrame(tc.rest)
		if path != tc.path || symbol != tc.symbol {
			t.Errorf("%q → (%q, %q), want (%q, %q)", tc.rest, path, symbol, tc.path, tc.symbol)
		}
	}
}

// Whatever an SDK sends as a trace, grouping must not panic and must give a hash.
func TestAppIssueFingerprintSurvivesHostileTraces(t *testing.T) {
	for _, trace := range []string{
		"NativeCrash", "NativeCrash:", "NativeCrash: \n", "AppHang:\n#", "AppHang:\n#00 pc", "AppHang:\n#00 pc zz /x",
		"NativeCrash:\n#00 pc 0 )))(((", "NativeCrash:\n#00 pc 0 /lib.so ()", "NativeCrash:\n#00 pc 0 /lib.so (+)",
		"WatchdogTermination:\n#00 " + strings.Repeat("a", 32) + " 0x /", "AppHang:\r\n\tat (",
		"ApplicationNotResponding:\n" + strings.Repeat("  at a.b(c)\n", 5000),
	} {
		if hash := ComputeExceptionHash(trace, false); len(hash) != 16 {
			t.Errorf("%q hashed to %q", trace, hash)
		}
	}
}
