package clientcontrollers

import (
	"regexp"
	"strings"
)

// The issue types the app SDKs read from what the OS recorded about a process: an Android
// tombstone or an iOS MetricKit crash, an Android ANR, a main-thread hang and an iOS watchdog kill. Their frames come
// from native code or from a sample of a blocked thread, so the frames below the app's own (the looper, the run loop,
// libart, libc) and the parts of a frame that differ per install, OS build or sample (install paths, build ids,
// program counters in system libraries) change between two occurrences of one problem. They group by their type and
// their top in-app frames instead of by the whole trace.
var frameGroupedIssueTypes = map[string]bool{
	"NativeCrash":              true,
	"ApplicationNotResponding": true,
	"AppHang":                  true,
	"WatchdogTermination":      true,
}

// fingerprintFrames is how many of the top in-app frames name the group: enough to tell two call sites apart, few
// enough that a hang sampled one call deeper or shallower stays in its group.
const fingerprintFrames = 5

var (
	// An Android tombstone or ANR native frame: `#00 pc 000000000004f8c8  /apex/…/libc.so (abort+164) (BuildId: …)`,
	// in an ANR dump prefixed with `native:`.
	nativeFrameRe = regexp.MustCompile(`^\s*(?:native:\s*)?#\d+\s+pc\s+([0-9a-fA-F]+)\s+(.+?)\s*$`)
	javaFrameRe   = regexp.MustCompile(`^\s*at\s+([\w$.<>-]+)\(([^)]*)\)\s*$`)
	// The iOS SDK's wire frame, before symbolication: `#00 <image uuid> 0x<offset in image> <image>`.
	iosWireFrameRe = regexp.MustCompile(`^\s*#\d+\s+([0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12})\s+0x([0-9a-fA-F]+)\s*(.*)$`)
	// Symbolicated iOS frames: `#3  ViewController.load() (ViewController.swift:42)`, or `#4  UIKitCore+0x1f2c`.
	iosResolvedFrameRe   = regexp.MustCompile(`^\s*#\d+\s+(.+?)\s+\((.+)\)\s*$`)
	iosUnresolvedFrameRe = regexp.MustCompile(`^\s*#\d+\s+(\S.*?)\+0x([0-9a-fA-F]+)\s*$`)
	symbolOffsetRe       = regexp.MustCompile(`\+(?:0x[0-9a-fA-F]+|\d+)$`)
	javaMethodRe         = regexp.MustCompile(`^[\w$]+(?:\.[\w$<>-]+)+$`)
	digitsRe             = regexp.MustCompile(`\d+`)

	javaSystemPrefixes = []string{
		"java.", "javax.", "jakarta.", "jdk.", "sun.", "com.sun.", "kotlin.", "kotlinx.", "android.", "androidx.",
		"com.android.", "com.google.android.", "dalvik.", "libcore.", "org.json.",
	}
	iosSystemImages = map[string]bool{
		"CoreFoundation": true, "Foundation": true, "CFNetwork": true, "Security": true, "UIKitCore": true,
		"UIKit": true, "SwiftUI": true, "GraphicsServices": true, "QuartzCore": true, "CoreGraphics": true,
		"CoreText": true, "CoreData": true, "CoreImage": true, "CoreAudio": true, "CoreVideo": true,
		"CoreServices": true, "CoreMotion": true, "CoreLocation": true, "Metal": true, "MetalKit": true,
		"ImageIO": true, "Combine": true, "Network": true, "AudioToolbox": true, "AVFoundation": true,
		"WebKit": true, "JavaScriptCore": true, "dyld": true, "FrontBoardServices": true, "BaseBoard": true,
		"RunningBoardServices": true, "UIKitServices": true, "SwiftUICore": true, "AttributeGraph": true,
	}
)

// frameGroupedIssueType returns the type of a trace whose first line names one of frameGroupedIssueTypes, as
// `AppHang: Main thread blocked for 3.2 s` or a bare `NativeCrash`.
func frameGroupedIssueType(stackTrace string) (string, bool) {
	first, _, _ := strings.Cut(stackTrace, "\n")
	typ, _, _ := strings.Cut(first, ":")
	typ = strings.TrimSpace(typ)
	return typ, frameGroupedIssueTypes[typ]
}

// appIssueFingerprint is what an issue of frameGroupedIssueTypes is grouped by: its type and its top in-app frames,
// else its top frames when none is the app's (an ANR on an idle looper), else its message without numbers.
func appIssueFingerprint(typ, stackTrace string) string {
	lines := strings.Split(strings.ReplaceAll(stackTrace, "\r\n", "\n"), "\n")
	var inApp, top []string
	for _, line := range lines[1:] {
		key, app, ok := fingerprintFrame(line)
		if !ok {
			continue
		}
		if len(top) < fingerprintFrames {
			top = append(top, key)
		}
		if app && len(inApp) < fingerprintFrames {
			inApp = append(inApp, key)
		}
	}
	frames := inApp
	if len(frames) == 0 {
		frames = top
	}
	if len(frames) == 0 {
		message := hexRe.ReplaceAllString(lines[0], "<hex>")
		message = digitsRe.ReplaceAllString(message, "<n>")
		return typ + "\n" + strings.Join(strings.Fields(message), " ")
	}
	return typ + "\n" + strings.Join(frames, "\n")
}

// fingerprintFrame reads one frame line as the part of it that is the same on every device running the same code,
// and whether the frame is the app's own.
func fingerprintFrame(line string) (key string, inApp bool, ok bool) {
	if m := nativeFrameRe.FindStringSubmatch(line); m != nil {
		path, symbol := splitNativeFrame(m[2])
		library := nativeLibraryName(path)
		inApp = isAppNativePath(path)
		if symbol != "" {
			if javaMethodRe.MatchString(symbol) && !strings.Contains(symbol, "::") {
				// Compiled or JIT-compiled Java code names its method; the class says whose it is.
				inApp = !isJavaSystemClass(symbol)
				return symbol, inApp, true
			}
			return library + " " + symbol, inApp, true
		}
		// Without a symbol the program counter is all that tells two frames apart. It moves with every build of the
		// library, which for the app's own libraries is a new release, as a line number is for Java frames.
		return library + " pc " + strings.TrimLeft(strings.ToLower(m[1]), "0"), inApp, true
	}
	if m := javaFrameRe.FindStringSubmatch(line); m != nil {
		return m[1], !isJavaSystemClass(m[1]), true
	}
	if m := iosWireFrameRe.FindStringSubmatch(line); m != nil {
		image := strings.TrimSpace(m[3])
		if image == "" || image == "<unknown>" {
			return "<unknown>", false, true
		}
		return image + "+0x" + strings.TrimLeft(strings.ToLower(m[2]), "0"), !isIOSSystemImage(image), true
	}
	if m := iosUnresolvedFrameRe.FindStringSubmatch(line); m != nil {
		image := strings.TrimSpace(m[1])
		return image + "+0x" + strings.TrimLeft(strings.ToLower(m[2]), "0"), !isIOSSystemImage(image), true
	}
	if m := iosResolvedFrameRe.FindStringSubmatch(line); m != nil {
		location := strings.TrimSpace(m[2])
		system := location == "" || location == "<compiler-generated>" || location == "<unknown>"
		return strings.TrimSpace(m[1]), !system, true
	}
	return "", false, false
}

// splitNativeFrame separates what follows a frame's program counter into the mapped file and the symbol, reading the
// parenthesised groups the unwinder appends from the end: `(BuildId: …)`, `(symbol+offset)`, `(offset 0x…)`, and a
// file's own `(deleted)`. A symbol may hold parentheses itself (`android::Looper::pollInner(int)+184`).
func splitNativeFrame(rest string) (path, symbol string) {
	rest = strings.TrimSpace(rest)
	for strings.HasSuffix(rest, ")") {
		open := matchingOpenParen(rest)
		if open <= 0 {
			break
		}
		group := strings.TrimSpace(rest[open+1 : len(rest)-1])
		before := strings.TrimSpace(rest[:open])
		switch {
		case strings.HasPrefix(group, "BuildId:"), strings.HasPrefix(group, "offset 0x"), group == "deleted":
		case symbol == "":
			symbol = symbolOffsetRe.ReplaceAllString(group, "")
		default:
			return rest, symbol
		}
		rest = before
	}
	return rest, symbol
}

func matchingOpenParen(s string) int {
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// nativeLibraryName keeps the file name of a mapped library: the directory an app is installed in is random per
// install (`/data/app/~~kXy1==/com.example-Qp9==/lib/arm64/`), and a library inside the APK reads `base.apk!libfoo.so`.
func nativeLibraryName(path string) string {
	if i := strings.LastIndexAny(path, "/!"); i >= 0 && i+1 < len(path) && !strings.HasPrefix(path, "[") {
		return path[i+1:]
	}
	return path
}

func isAppNativePath(path string) bool {
	return strings.HasPrefix(path, "/data/") || strings.HasPrefix(path, "/mnt/expand/")
}

func isJavaSystemClass(name string) bool {
	for _, prefix := range javaSystemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func isIOSSystemImage(image string) bool {
	if image == "" || image == "<unknown>" || iosSystemImages[image] {
		return true
	}
	return strings.HasPrefix(image, "libswift") || strings.HasPrefix(image, "lib") && strings.HasSuffix(image, ".dylib")
}
