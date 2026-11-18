package seed_test

import (
	"context"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
	client "github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

// TestNativeCrashWithActions sends a mobile crash with captured network
// actions over /api/report and reads it back as an agent would.
func TestNativeCrashWithActions(t *testing.T) {
	bins := env.BinariesForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	inst, err := env.NewInstance(ctx, bins, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	s := seed.NewScenario("native-test", time.Now(), seed.NewLiveSink(ctx, inst.Backend, inst.Admin))
	p := s.Project("shop-android", seed.Mobile, "android")
	at := s.Ago(2 * time.Hour)
	p.Crash(seed.Crash{At: at, AppVersion: "4.12.0", Device: "Pixel 8",
		Stack:      "java.net.SocketTimeoutException: timeout\n\tat okhttp3.internal.http2.Http2Stream$StreamTimeout.newTimeoutException(Http2Stream.kt:675)\n\tat com.example.shop.checkout.CheckoutRepository.placeOrder(CheckoutRepository.kt:88)",
		Attributes: map[string]string{"device.model": "Pixel 8", "os.version": "15"},
		Actions:    []seed.Action{{Type: "network", At: at.Add(-10 * time.Second), Method: "POST", URL: "https://api.shop.example/api/checkout", DurationMs: 10012, Error: "timeout"}},
	})
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	c := client.New(inst.Backend.URL, client.WithJWT(inst.Admin.JWT))
	list, err := c.ListExceptions(ctx, p.ID, client.ListExceptionsRequest{TimeRange: client.TimeRangeFromExplicit(s.Ago(24*time.Hour), s.Now.Add(time.Hour)), Pagination: client.PaginationParams{Page: 1, PageSize: 10}})
	if err != nil || len(list.Data) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	g, err := c.GetException(ctx, p.ID, list.Data[0].ExceptionHash, client.PaginationParams{Page: 1, PageSize: 5})
	if err != nil || len(g.Occurrences) != 1 {
		t.Fatalf("show: %v", err)
	}
	var occ *client.ExceptionByIdResponse
	for i := 0; i < 40; i++ {
		occ, err = c.GetExceptionById(ctx, p.ID, g.Occurrences[0].Id.String(), g.Occurrences[0].RecordedAt)
		if err != nil {
			t.Fatal(err)
		}
		if len(occ.SessionRecording) > 0 && string(occ.SessionRecording) != "null" {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if len(occ.SessionRecording) == 0 || string(occ.SessionRecording) == "null" {
		t.Fatalf("the occurrence carries no recording with the captured actions: %+v", occ)
	}
	t.Logf("recording: %.300s", string(occ.SessionRecording))
}
