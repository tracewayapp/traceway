package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/android-npe", dbgSeedAndroidNPE) }

const dbgRecyclerFrames = "\tat androidx.recyclerview.widget.RecyclerView$Adapter.onBindViewHolder(RecyclerView.java:7254)\n" +
	"\tat androidx.recyclerview.widget.RecyclerView$Adapter.bindViewHolder(RecyclerView.java:7337)\n" +
	"\tat androidx.recyclerview.widget.RecyclerView$Recycler.tryBindViewHolderByDeadline(RecyclerView.java:6194)\n" +
	"\tat androidx.recyclerview.widget.RecyclerView$Recycler.tryGetViewHolderForPositionByDeadline(RecyclerView.java:6427)\n" +
	"\tat androidx.recyclerview.widget.GapWorker.prefetchPositionWithDeadline(GapWorker.java:288)\n" +
	"\tat androidx.recyclerview.widget.GapWorker.flushTaskWithDeadline(GapWorker.java:345)\n" +
	"\tat androidx.recyclerview.widget.GapWorker.flushTasksWithDeadline(GapWorker.java:361)\n" +
	"\tat androidx.recyclerview.widget.GapWorker.prefetch(GapWorker.java:368)\n" +
	"\tat androidx.recyclerview.widget.GapWorker.run(GapWorker.java:399)\n" +
	"\tat android.os.Handler.handleCallback(Handler.java:958)\n" +
	"\tat android.os.Handler.dispatchMessage(Handler.java:99)\n" +
	"\tat android.os.Looper.loopOnce(Looper.java:205)\n" +
	"\tat android.os.Looper.loop(Looper.java:294)\n" +
	"\tat android.app.ActivityThread.main(ActivityThread.java:8177)\n" +
	"\tat java.lang.reflect.Method.invoke(Native Method)\n" +
	"\tat com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:552)\n" +
	"\tat com.android.internal.os.ZygoteInit.main(ZygoteInit.java:971)\n"

// dbgSeedAndroidNPE: the Android app's 5.8.0 (staged rollout since yesterday)
// made FeedItem.image non-null and sizes thumbnails from item.image.width;
// Gson leaves image null for text posts, so binding one throws an NPE while
// the feed scrolls (RecyclerView prefetch). Traces arrive deobfuscated
// (R8 mapping uploaded). Distractor: the full-size image viewer runs out of
// memory decoding originals on low-memory phones, on every version, for
// weeks.
func dbgSeedAndroidNPE(s *seed.Scenario) error {
	p := s.Project("pinboard-android", seed.Mobile, "android")
	npe := "java.lang.NullPointerException: Attempt to invoke virtual method 'int com.example.pinboard.feed.ImageRef.getWidth()' on a null object reference\n" +
		"\tat com.example.pinboard.feed.FeedAdapter.bindImage(FeedAdapter.kt:55)\n" +
		"\tat com.example.pinboard.feed.FeedAdapter.onBindViewHolder(FeedAdapter.kt:48)\n" +
		"\tat com.example.pinboard.feed.FeedAdapter.onBindViewHolder(FeedAdapter.kt:21)\n" + dbgRecyclerFrames
	oom := "java.lang.OutOfMemoryError: Failed to allocate a 96000012 byte allocation with 25165824 free bytes and 61MB until OOM, target footprint 268435456, growth limit 268435456\n" +
		"\tat dalvik.system.VMRuntime.newNonMovableArray(Native Method)\n" +
		"\tat android.graphics.Bitmap.nativeCreate(Native Method)\n" +
		"\tat android.graphics.BitmapFactory.nativeDecodeStream(Native Method)\n" +
		"\tat android.graphics.BitmapFactory.decodeStreamInternal(BitmapFactory.java:875)\n" +
		"\tat android.graphics.BitmapFactory.decodeStream(BitmapFactory.java:851)\n" +
		"\tat com.example.pinboard.media.FullImageActivity.decodeOriginal(FullImageActivity.kt:53)\n" +
		"\tat com.example.pinboard.media.FullImageActivity.download(FullImageActivity.kt:43)\n" +
		"\tat com.example.pinboard.media.FullImageActivity.access$download(FullImageActivity.kt:19)\n" +
		"\tat com.example.pinboard.media.FullImageActivity$onCreate$1$bitmap$1.invokeSuspend(FullImageActivity.kt:34)\n" +
		"\tat kotlin.coroutines.jvm.internal.BaseContinuationImpl.resumeWith(ContinuationImpl.kt:33)\n" +
		"\tat kotlinx.coroutines.DispatchedTask.run(DispatchedTask.kt:104)\n" +
		"\tat kotlinx.coroutines.internal.LimitedDispatcher$Worker.run(LimitedDispatcher.kt:111)\n" +
		"\tat kotlinx.coroutines.scheduling.TaskImpl.run(Tasks.kt:99)\n" +
		"\tat kotlinx.coroutines.scheduling.CoroutineScheduler.runSafely(CoroutineScheduler.kt:584)\n" +
		"\tat kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.executeTask(CoroutineScheduler.kt:811)\n" +
		"\tat kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.runWorker(CoroutineScheduler.kt:715)\n" +
		"\tat kotlinx.coroutines.scheduling.CoroutineScheduler$Worker.run(CoroutineScheduler.kt:702)\n"
	modern := []string{"Pixel 8", "Pixel 7a", "SM-S921B", "SM-A546B", "moto g84 5G", "2312DRA50G", "Pixel 9 Pro"}
	lowMem := []string{"SM-A135F", "moto e13", "Redmi 10C", "SM-A045F"}

	// 5.8.0 rolls out from 26h ago; crash volume grows with the rollout.
	for _, at := range dbgTimes(s, s.Ago(24*time.Hour), s.Ago(4*time.Minute), 84, true) {
		c := seed.Crash{At: at, Stack: npe, AppVersion: "5.8.0", Device: dbgPick(s, modern)}
		if at.After(s.Ago(10 * time.Hour)) {
			cursor := fmt.Sprintf("c_%x", s.RNG.IntN(1<<30))
			first, firstMs := at.Add(-time.Duration(7800+s.RNG.IntN(900))*time.Millisecond), 180+s.RNG.IntN(300)
			next, nextMs := at.Add(-time.Duration(900+s.RNG.IntN(1200))*time.Millisecond), 160+s.RNG.IntN(300)
			loaded := func(req time.Time, ms int) time.Time {
				return req.Add(time.Duration(ms+20+s.RNG.IntN(60)) * time.Millisecond)
			}
			// The SDK attaches only the last 10 seconds of actions and logs.
			c.Actions = []seed.Action{
				{Type: "navigation", At: first.Add(-time.Duration(300+s.RNG.IntN(700)) * time.Millisecond), From: "SplashActivity", To: "FeedActivity"},
				{Type: "network", At: first, Method: "GET", URL: "https://api.pinboard.example/v2/feed?limit=20", DurationMs: float64(firstMs), Status: 200, RespBytes: 18000 + s.RNG.IntN(9000)},
				{Type: "network", At: next, Method: "GET", URL: "https://api.pinboard.example/v2/feed?cursor=" + cursor + "&limit=20", DurationMs: float64(nextMs), Status: 200, RespBytes: 17000 + s.RNG.IntN(9000)},
			}
			c.Logs = []seed.ActionLog{
				{At: loaded(first, firstMs), Level: "info", Message: "feed page loaded: 20 items"},
				{At: loaded(next, nextMs), Level: "info", Message: "feed page loaded: 20 items"},
			}
		}
		p.Crash(c)
	}
	versions := []string{"5.6.4", "5.7.1", "5.7.2", "5.7.2", "5.8.0"}
	for _, at := range dbgTimes(s, s.Ago(7*24*time.Hour), s.Ago(40*time.Minute), 26, false) {
		v := dbgPick(s, versions)
		if v == "5.8.0" && at.Before(s.Ago(24*time.Hour)) {
			v = "5.7.2"
		}
		p.Crash(seed.Crash{At: at, Stack: oom, AppVersion: v, Device: dbgPick(s, lowMem)})
	}

	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "FeedAdapter.bindImage")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "OutOfMemoryError")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		return nil
	})
	return nil
}
