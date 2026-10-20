package debug

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/ios-force-unwrap", dbgSeedIOSForceUnwrap) }

const dbgUIKitTail = "#4  SwiftUI+0x9b1c24\n#5  SwiftUI+0x9b2310\n#6  SwiftUI+0x16e4f0\n#7  SwiftUI+0x8c7d38\n#8  SwiftUI+0x8c80a4\n" +
	"#9  UIKitCore+0x1a0b5c\n#10  UIKitCore+0x1a09f8\n#11  CoreFoundation+0x7a4c4\n#12  CoreFoundation+0x7a0b8\n#13  GraphicsServices+0x1998\n" +
	"#14  UIKitCore+0x3c2a94\n#15  SwiftUI+0x1ee1d8\n#16  static ShopApp.$main() (ShopApp.swift:4:1)\n#17  dyld+0x5d44"

// dbgSeedIOSForceUnwrap: 3.12.0 (released yesterday) added live chat to the
// Help center and force-unwraps the support chat URL from remote config,
// which is not set for every audience; tapping "Chat with us" there traps.
// Traces arrive symbolicated (dSYMs uploaded). Distractor: an older
// NSInternalInconsistencyException from the cart's batch updates.
func dbgSeedIOSForceUnwrap(s *seed.Scenario) error {
	p := s.Project("shop-ios", seed.Mobile, "ios")
	trap := "Fatal Signal SIGTRAP (5)\n" +
		"#0  HelpCenterViewModel.chatURL.getter (HelpCenterViewModel.swift:23:41)\n" +
		"#1  closure #4 in HelpCenterView.body.getter (HelpCenterView.swift:24:35)\n" +
		"#2  SwiftUI+0x6f1e08\n#3  SwiftUI+0x6f2a54\n" + dbgUIKitTail
	batch := "NSInternalInconsistencyException: Invalid batch updates detected: the number of sections and/or items returned by the data source before and after performing the batch updates are inconsistent with the updates.\n" +
		"#0  CoreFoundation+0x4a9d4\n#1  libobjc.A.dylib+0x31e8\n#2  Foundation+0x8e4c30\n#3  UIKitCore+0x4f1a3c\n" +
		"#4  CartViewController.apply(_:) (CartViewController.swift:51:28)\n" +
		"#5  closure #1 in CartViewController.removeItem(at:) (CartViewController.swift:40:13)\n" +
		"#6  libswift_Concurrency.dylib+0x5b6dc\n#7  libdispatch.dylib+0x3fdc\n#8  libdispatch.dylib+0x1b5f4\n" +
		"#9  CoreFoundation+0x7a4c4\n#10  CoreFoundation+0x7a0b8\n#11  GraphicsServices+0x1998\n#12  UIKitCore+0x3c2a94\n" +
		"#13  SwiftUI+0x1ee1d8\n#14  static ShopApp.$main() (ShopApp.swift:4:1)\n#15  dyld+0x5d44"
	devices := []string{"iPhone15,2", "iPhone15,3", "iPhone14,7", "iPhone16,1", "iPhone16,2", "iPhone13,2", "iPhone17,1"}

	// 3.12.0 shipped about 25h ago; adoption (and crashes) grow since.
	for _, at := range dbgTimes(s, s.Ago(23*time.Hour), s.Ago(7*time.Minute), 58, true) {
		p.Crash(seed.Crash{At: at, Stack: trap, AppVersion: "3.12.0", Device: dbgPick(s, devices)})
	}
	versions := []string{"3.10.2", "3.11.4", "3.11.4", "3.12.0"}
	for _, at := range dbgTimes(s, s.Ago(7*24*time.Hour), s.Ago(3*time.Hour), 14, false) {
		v := dbgPick(s, versions)
		if v == "3.12.0" && at.Before(s.Ago(23*time.Hour)) {
			v = "3.11.4"
		}
		p.Crash(seed.Crash{At: at, Stack: batch, AppVersion: v, Device: dbgPick(s, devices)})
	}

	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "HelpCenterViewModel.chatURL")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "NSInternalInconsistencyException")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		return nil
	})
	return nil
}
