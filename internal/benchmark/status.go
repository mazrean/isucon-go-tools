package benchmark

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gobFile    string
	latest     = &Benchmark{}
	scoreGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "isutools",
		Subsystem: "benchmark",
		Name:      "score",
	})
	durationGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "isutools",
		Subsystem: "benchmark",
		Name:      "duration",
	})
)

type Benchmark struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Score int64     `json:"score"`
}

func init() {
	if !config.Enable {
		return
	}

	var ok bool
	gobFile, ok = os.LookupEnv("BENCHMARK_FILE")
	if !ok {
		gobFile = "bench.gob"
	}

	f, err := os.Open(gobFile)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("failed to open gob file",
				slog.String("file", gobFile),
				slog.String("error", err.Error()),
			)
		}

		return
	}
	defer f.Close()

	err = gob.NewDecoder(f).Decode(latest)
	if err != nil {
		slog.Error("failed to decode gob file",
			slog.String("file", gobFile),
			slog.String("error", err.Error()),
		)
	}

	scoreGauge.Set(float64(latest.Score))
	durationGauge.Set(latest.End.Sub(latest.Start).Seconds())
}

var (
	start time.Time
	end   = atomic.Pointer[time.Time]{}
)

func Start() {
	start = time.Now()
}

func Continue() {
	v := time.Now().Add(2 * time.Second)
	end.Store(&v)
}

var (
	endHooks []func(context.Context, *Benchmark)
)

func SetEndHook(f func(context.Context, *Benchmark)) {
	endHooks = append(endHooks, f)
}

func setScore(ctx context.Context, score int64) {
	end := end.Load()
	if end == nil {
		slog.Error("end time is not set",
			slog.Time("start", start),
			slog.Int64("score", score),
		)
		return
	}

	latest = &Benchmark{
		Start: start,
		End:   *end,
		Score: score,
	}
	scoreGauge.Set(float64(score))
	durationGauge.Set(latest.End.Sub(latest.Start).Seconds())

	for _, f := range endHooks {
		f(ctx, latest)
	}

	f, err := os.Create(gobFile)
	if err != nil {
		slog.Error("failed to create gob file",
			slog.String("file", gobFile),
			slog.String("error", err.Error()),
		)
	}
	defer f.Close()

	err = gob.NewEncoder(f).Encode(latest)
	if err != nil {
		slog.Error("failed to encode gob file",
			slog.String("file", gobFile),
			slog.Group("latest",
				slog.Time("start", latest.Start),
				slog.Time("end", latest.End),
				slog.Int64("score", latest.Score),
			),
			slog.String("error", err.Error()),
		)
	}
}

func Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /benchmark/score", func(w http.ResponseWriter, r *http.Request) {
		strScore := r.FormValue("score")
		score, err := strconv.ParseInt(strScore, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to parse score(%s): %s", strScore, err), http.StatusBadRequest)
			return
		}

		setScore(r.Context(), score)

		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /benchmark/latest", func(w http.ResponseWriter, r *http.Request) {
		if latest == nil {
			http.Error(w, "no latest benchmark", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(latest)
	})
}
