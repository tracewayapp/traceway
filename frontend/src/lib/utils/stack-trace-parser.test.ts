import { describe, expect, it } from 'vitest';
import {
	looksLikeJava,
	looksLikeNative,
	parseNativeFrame,
	parseStackTrace,
	type FrameGroup
} from './stack-trace-parser';

// An Android 14 tombstone of a crash in the app's own native library, as the Android SDK reports it.
const TOMBSTONE = `NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0000000000000000
Cause: null pointer dereference
    #00 pc 000000000001c6a4  /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/lib/arm64/libpayments.so (pay::Card::charge(int)+52) (BuildId: 6f2c1a9e8b7d4c3a2f1e0d9c8b7a6f5e4d3c2b1a)
    #01 pc 000000000001b210  /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/lib/arm64/libpayments.so (Java_com_example_shop_Payments_nativeCharge+96) (BuildId: 6f2c1a9e8b7d4c3a2f1e0d9c8b7a6f5e4d3c2b1a)
    #02 pc 00000000002d4044  /apex/com.android.art/lib64/libart.so (art_quick_generic_jni_trampoline+148) (BuildId: 9e7c7dcb2a5b9f0c3e2a4f8a1b6d0c7e)
    #03 pc 00000000002ca9e8  /apex/com.android.art/lib64/libart.so (art_quick_invoke_stub+568) (BuildId: 9e7c7dcb2a5b9f0c3e2a4f8a1b6d0c7e)
    #04 pc 00000000002ef3a8  /apex/com.android.art/lib64/libart.so (art::ArtMethod::Invoke(art::Thread*, unsigned int*, unsigned int, art::JValue*, char const*)+216) (BuildId: 9e7c7dcb2a5b9f0c3e2a4f8a1b6d0c7e)
    #05 pc 0000000000412b4c  /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/oat/arm64/base.odex (com.example.shop.Payments.charge+124)
    #06 pc 00000000005a1c30  /system/framework/arm64/boot-framework.oat (android.view.View.performClick+168) (BuildId: 2b9a1c3d4e5f60718293a4b5c6d7e8f901234567)
    #07 pc 0000000000216e8c  /apex/com.android.runtime/lib64/bionic/libc.so (__start_thread+64) (BuildId: 1c3a2b5d4e6f7a8b9c0d1e2f3a4b5c6d)`;

// An ANR dump of the main thread, blocked on a lock in the app.
const ANR = `ApplicationNotResponding: Input dispatching timed out (com.example.shop/com.example.shop.CheckoutActivity, Waited 5001ms for MotionEvent)
"main" prio=5 tid=1 Blocked
  | group="main" sCount=1 ucsCount=0 flags=1 obj=0x72e3b8f8 self=0xb4000077f3c1b7b0
  | sysTid=8123 nice=-10 cgrp=top-app sched=0/0 handle=0x790e6b44f8
  native: #00 pc 00000000000a33b8  /apex/com.android.runtime/lib64/bionic/libc.so (syscall+28) (BuildId: 1c3a2b5d4e6f7a8b9c0d1e2f3a4b5c6d)
  native: #01 pc 0000000000470d34  /apex/com.android.art/lib64/libart.so (art::Monitor::Lock(art::Thread*)+412) (BuildId: 9e7c7dcb2a5b9f0c3e2a4f8a1b6d0c7e)
  at com.example.shop.cart.CartRepository.load(CartRepository.kt:41)
  - waiting to lock <0x0d3a1f2e> (a java.lang.Object) held by thread 23
  at com.example.shop.cart.CartViewModel.refresh(CartViewModel.kt:67)
  at android.app.Activity.performResume(Activity.java:8422)
  at android.os.Looper.loop(Looper.java:288)`;

// An iOS hang in the SDK's wire format, before (or without) symbolication.
const IOS_HANG = `AppHang: Main thread blocked for 3.2 s
*** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
os: ios arch: arm64
#00 6f1c2a3b4c5d6e7f8091a2b3c4d5e6f7 0x1d2c4 libsystem_kernel.dylib
#01 c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6 0x3a1f0 Foundation
#02 0a1b2c3d4e5f60718293a4b5c6d7e8f9 0x1a2b4 ShopApp
#03 d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9 0x2f4e10 UIKitCore
#04 6F1C2A3B-4C5D-6E7F-8091-A2B3C4D5E6F7 0x8c40 ShopApp`;

function shape(groups: FrameGroup[]) {
	return groups.map((g) =>
		g.type === 'app'
			? `app ${g.frame.functionName} @ ${g.frame.location}`
			: `${g.frames.length} × ${g.packageName}`
	);
}

describe('parseNativeFrame', () => {
	it('reads the file, the symbol and its offset of every unwinder layout', () => {
		expect(
			parseNativeFrame(
				'    #00 pc 000000000004f8c8  /apex/com.android.runtime/lib64/bionic/libc.so (abort+164) (BuildId: 1c3a)'
			)
		).toEqual({
			pc: '000000000004f8c8',
			path: '/apex/com.android.runtime/lib64/bionic/libc.so',
			symbol: 'abort',
			offset: '164'
		});
		expect(
			parseNativeFrame(
				'  native: #01 pc 0000000000010dfc  /system/lib64/libutils.so (android::Looper::pollOnce(int, int*, int*, void**)+156) (BuildId: 8a9b)'
			)
		).toMatchObject({
			symbol: 'android::Looper::pollOnce(int, int*, int*, void**)',
			offset: '156'
		});
		expect(
			parseNativeFrame(
				'#00 pc 000000000011d004  /data/app/x/base.apk!libimage.so (offset 0x2c4000) (BuildId: 3e4f)'
			)
		).toEqual({
			pc: '000000000011d004',
			path: '/data/app/x/base.apk!libimage.so',
			symbol: null,
			offset: null
		});
		expect(
			parseNativeFrame(
				'#02 pc 0000000000002f40  /memfd:jit-cache (deleted) (offset 0x2000000) (com.example.Decoder.decode+512)'
			)
		).toMatchObject({ path: '/memfd:jit-cache', symbol: 'com.example.Decoder.decode' });
		expect(parseNativeFrame('#03 pc 00000000000012ab  [anon:dalvik-jit-code-cache]')).toMatchObject(
			{ path: '[anon:dalvik-jit-code-cache]', symbol: null }
		);
	});

	it('takes no other line for a native frame', () => {
		for (const line of [
			'#0      main (package:shop/main.dart:5:3)',
			'\tat com.example.Foo.bar(Foo.kt:1)',
			'#00 2dd71042118432be8f92dd4e3d3fe24a 0x460 sample',
			'#00 pc',
			'#00 pc zz /x',
			''
		]) {
			expect(parseNativeFrame(line)).toBeNull();
		}
	});
});

describe('parseStackTrace with native frames', () => {
	it('groups a tombstone into the app’s frames and the platform’s libraries', () => {
		expect(looksLikeNative(TOMBSTONE)).toBe(true);
		expect(looksLikeJava(TOMBSTONE)).toBe(false);
		const parsed = parseStackTrace(TOMBSTONE, { native: true });
		expect(parsed.errorMessage).toBe(
			'NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0000000000000000\nCause: null pointer dereference'
		);
		expect(shape(parsed.groups)).toEqual([
			'app pay::Card::charge(int)+52 @ /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/lib/arm64/libpayments.so',
			'app Java_com_example_shop_Payments_nativeCharge+96 @ /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/lib/arm64/libpayments.so',
			'3 × libart.so',
			'app com.example.shop.Payments.charge+124 @ /data/app/~~Gf3k2Lq9xT1VbN0mZ4wQyA==/com.example.shop-Hn7dR2pX5sK8LmC0vB1zQw==/oat/arm64/base.odex',
			'2 × android'
		]);
	});

	it('reads an ANR dump: native and Java frames, the lock the main thread waits for', () => {
		expect(looksLikeNative(ANR) && looksLikeJava(ANR)).toBe(true);
		const parsed = parseStackTrace(ANR, { native: true, java: true });
		expect(parsed.errorMessage).toBe(
			'ApplicationNotResponding: Input dispatching timed out (com.example.shop/com.example.shop.CheckoutActivity, Waited 5001ms for MotionEvent)\n"main" prio=5 tid=1 Blocked'
		);
		expect(shape(parsed.groups)).toEqual([
			'2 × libc.so',
			'app CartRepository.load @ CartRepository.kt:41',
			'app CartViewModel.refresh @ CartViewModel.kt:67',
			'2 × android'
		]);
		const blocked = parsed.groups[1];
		expect(blocked.type === 'app' && blocked.frame.note).toBe(
			'waiting to lock <0x0d3a1f2e> (a java.lang.Object) held by thread 23'
		);
	});

	it('reads the iOS wire format and leaves its preamble out of the message', () => {
		const parsed = parseStackTrace(IOS_HANG, { ios: true });
		expect(parsed.errorMessage).toBe('AppHang: Main thread blocked for 3.2 s');
		expect(shape(parsed.groups)).toEqual([
			'2 × libsystem_kernel.dylib',
			'app null @ ShopApp+0x1a2b4',
			'1 × UIKitCore',
			'app null @ ShopApp+0x8c40'
		]);
	});

	it('keeps the lines of another platform’s message that look like an iOS preamble or an ART thread state', () => {
		const raw =
			'Error: query failed\n| id | name |\n*** *** ***\nloadUsers()\n    src/users.ts:10:5';
		expect(parseStackTrace(raw).errorMessage).toBe(
			'Error: query failed\n| id | name |\n*** *** ***'
		);
	});

	it('survives whatever an SDK sends', () => {
		for (const raw of [
			'',
			'NativeCrash',
			'NativeCrash:\n#00 pc 0 )))(((',
			'NativeCrash:\n#00 pc 0 /lib.so ()',
			'NativeCrash:\n  native: #00 pc 1 (x+1) (y+2) (z+3)',
			'ApplicationNotResponding:\n  - waiting to lock <0x1>\n  at a.b(c)',
			'AppHang:\n#00 ' + 'a'.repeat(32) + ' 0x /',
			'AppHang:\n' + '#00 pc 0 /x.so (f+1)\n'.repeat(2000)
		]) {
			expect(() => parseStackTrace(raw, { native: true, java: true, ios: true })).not.toThrow();
		}
	});
});
