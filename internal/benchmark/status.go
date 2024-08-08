package benchmark

import (
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	gobFile string
	latest  *Benchmark
)

type Benchmark struct {
	Start time.Time
	End   time.Time
	Score int64
}

func init() {
	var ok bool
	gobFile, ok = os.LookupEnv("BENCHMARK_FILE")
	if !ok {
		gobFile = "bench.gob"
	}

	f, err := os.Open(gobFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to open gob file(%s): %s\n", gobFile, err)
		}

		return
	}
	defer f.Close()

	err = gob.NewDecoder(f).Decode(latest)
	if err != nil {
		log.Printf("failed to decode gob file(%s): %s\n", gobFile, err)
	}
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
	latest = &Benchmark{
		Start: start,
		End:   *end.Load(),
		Score: score,
	}

	f, err := os.Create(gobFile)
	if err != nil {
		log.Printf("failed to create gob file(%s): %s\n", gobFile, err)
		return
	}

	err = gob.NewEncoder(f).Encode(latest)
	if err != nil {
		log.Printf("failed to encode gob file(%s): %s\n", gobFile, err)
	}

	for _, f := range endHooks {
		f(ctx, latest)
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
	})
}
