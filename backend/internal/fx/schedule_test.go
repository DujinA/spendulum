package fx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

// A synctest bubble starts at 2000-01-01 00:00 UTC, which is 01:00 in Berlin (CET, UTC+1).

func TestTheDailySyncFetchesTheNewestRatesAt1700Berlin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFixture()

		runScheduler(t, f, 2*24*time.Hour+time.Hour)

		want := []time.Time{
			utc(t, "2000-01-01T00:00:00Z"), // the startup backfill
			utc(t, "2000-01-01T16:00:00Z"),
			utc(t, "2000-01-02T16:00:00Z"),
		}
		if got := slices.Compact(f.provider.callTimes()); !reflect.DeepEqual(got, want) {
			t.Errorf("runs at %v, want %v", got, want)
		}

		if got := f.provider.madeRequests(); got[len(got)-1].to != day(t, "2000-01-02") {
			t.Errorf("the last sync asked for rates through %s, want the newest, 2000-01-02", got[len(got)-1].to)
		}
	})
}

func TestTheDailySyncFollowsBerlinSummerTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFixture()

		runScheduler(t, f, 87*24*time.Hour)

		want := []time.Time{
			utc(t, "2000-03-25T16:00:00Z"),
			utc(t, "2000-03-26T15:00:00Z"), // summer time started at 02:00 that night
			utc(t, "2000-03-27T15:00:00Z"),
		}
		if got := slices.Compact(f.provider.callTimes()); !reflect.DeepEqual(got[len(got)-3:], want) {
			t.Errorf("last three syncs at %v, want %v", got[len(got)-3:], want)
		}
	})
}

func TestAFailedSyncRetriesWithBackoffUntilTheNextScheduledRunAndIsLogged(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFixture()
		f.provider.fail(errProviderDown)

		logs := runScheduler(t, f, 16*time.Hour+5*time.Minute)

		got := f.provider.callTimes()
		first := []time.Time{
			utc(t, "2000-01-01T00:00:00Z"),
			utc(t, "2000-01-01T00:01:00Z"),
			utc(t, "2000-01-01T00:03:00Z"),
			utc(t, "2000-01-01T00:07:00Z"),
			utc(t, "2000-01-01T00:15:00Z"),
			utc(t, "2000-01-01T00:31:00Z"),
			utc(t, "2000-01-01T01:03:00Z"),
			utc(t, "2000-01-01T02:03:00Z"),
			utc(t, "2000-01-01T03:03:00Z"),
		}
		around17 := []time.Time{
			utc(t, "2000-01-01T15:03:00Z"),
			utc(t, "2000-01-01T16:00:00Z"), // the next scheduled run takes over
			utc(t, "2000-01-01T16:01:00Z"),
			utc(t, "2000-01-01T16:03:00Z"),
		}

		if len(got) != 24 || !reflect.DeepEqual(got[:9], first) || !reflect.DeepEqual(got[20:], around17) {
			t.Fatalf("provider asked at %v,\nwant it to start with %v\nand end with %v", got, first, around17)
		}

		failures := logs.records("exchange-rate sync failed")
		if len(failures) != len(got) {
			t.Fatalf("%d failures logged, want one per attempt (%d)", len(failures), len(got))
		}

		if failures[0]["level"] != "ERROR" || !strings.Contains(failures[0]["error"].(string), errProviderDown.Error()) {
			t.Errorf("failure logged as %v, want an ERROR naming the provider's error", failures[0])
		}
	})
}

func TestTheDailyRunBackfillsTheHistoryAFailedStartupLeftMissing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFixture()
		f.entries.earliest = day(t, "1999-12-01")

		for d := day(t, "1999-11-20"); !d.After(day(t, "2000-01-01")); d = d.AddDate(0, 0, 1) {
			f.provider.rates = append(f.provider.rates, fx.Rate{Currency: "RSD", Date: d, PerEUR: big.NewRat(1175, 10)})
		}

		f.provider.fail(errProviderDown)
		stop := startScheduler(t, f)

		time.Sleep(15*time.Hour + 30*time.Minute) // after the last startup retry, at 15:03 UTC
		f.provider.fail(nil)
		time.Sleep(time.Hour) // past the 17:00 Berlin run
		stop()

		if _, err := f.svc.Convert(t.Context(), 100, "EUR", "RSD", day(t, "1999-12-01")); err != nil {
			t.Errorf("Convert on the earliest entry date after the 17:00 run: %v", err)
		}
	})
}

// runScheduler runs the scheduler for d of bubble time, stops it, and returns what it logged.
func runScheduler(t *testing.T, f *fixture, d time.Duration) *logBuffer {
	t.Helper()

	stop := startScheduler(t, f)
	time.Sleep(d)

	return stop()
}

func startScheduler(t *testing.T, f *fixture) (stop func() *logBuffer) {
	t.Helper()

	logs := &logBuffer{}

	s, err := fx.NewScheduler(f.svc, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		s.Run(ctx)
		close(done)
	}()

	return func() *logBuffer {
		cancel()
		<-done

		return logs
	}
}

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *logBuffer) records(msg string) []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()

	var out []map[string]any

	for line := range strings.Lines(b.buf.String()) {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) == nil && rec["msg"] == msg {
			out = append(out, rec)
		}
	}

	return out
}

func utc(tb testing.TB, s string) time.Time {
	tb.Helper()

	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		tb.Fatal(err)
	}

	return ts
}
