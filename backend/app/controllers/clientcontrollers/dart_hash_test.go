package clientcontrollers

import "testing"

func TestDartSymbolicatedHashIsMachineIndependent(t *testing.T) {
	alice := "PaymentDeclinedException: card declined for $30.59\n" +
		"#0  chargeCard (/Users/alice/app/lib/main.dart:20:3)\n" +
		"#1  applyTax (/Users/alice/app/lib/main.dart:30:10)\n" +
		"#2  checkout (/Users/alice/app/lib/main.dart:35:10)"

	bob := "PaymentDeclinedException: card declined for $42.00\n" +
		"#0  chargeCard (/home/bob/ci/workspace/lib/main.dart:20:3)\n" +
		"#1  applyTax (/home/bob/ci/workspace/lib/main.dart:30:10)\n" +
		"#2  checkout (/home/bob/ci/workspace/lib/main.dart:35:10)"

	if ComputeExceptionHash(alice, false) != ComputeExceptionHash(bob, false) {
		t.Errorf("same crash on different machines should hash the same")
	}

	other := "PaymentDeclinedException: card declined for $30.59\n" +
		"#0  refund (/Users/alice/app/lib/main.dart:88:3)"
	if ComputeExceptionHash(alice, false) == ComputeExceptionHash(other, false) {
		t.Errorf("different stacks should not share a hash")
	}
}

func TestDartOffsetFrameHashStable(t *testing.T) {

	report1 := "PaymentDeclinedException: card declined for $30.59\n" +
		"#0  _kDartIsolateSnapshotInstructions+141e6b\n" +
		"#1  _kDartIsolateSnapshotInstructions+141d9b"
	report2 := "PaymentDeclinedException: card declined for $99.10\n" +
		"#0  _kDartIsolateSnapshotInstructions+141e6b\n" +
		"#1  _kDartIsolateSnapshotInstructions+141d9b"
	if ComputeExceptionHash(report1, false) != ComputeExceptionHash(report2, false) {
		t.Errorf("same crash (same offsets, different message values) should hash the same")
	}
}
