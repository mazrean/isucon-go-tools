package profiler

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/pyroscope-go"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/querier/v1/querierv1connect"
	connectapi "github.com/grafana/pyroscope/pkg/api/connect"
	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
)

var (
	serverAddr  string
	pgoFile     string
	profileType string
	query       string
)

func setupPyroscope() error {
	var ok bool
	serverAddr, ok = os.LookupEnv("PYROSCOPE_SERVER")
	if !ok {
		serverAddr = "http://0.0.0.0:4040"
	}

	pgoFile, ok = os.LookupEnv("PGO_FILE")
	if !ok {
		pgoFile = "default.pgo"
	}

	profileType, ok = os.LookupEnv("PGO_PROFILE_TYPE")
	if !ok {
		profileType = "process_cpu:cpu:nanoseconds:cpu:nanoseconds"
	}

	query, ok = os.LookupEnv("PGO_QUERY")
	if !ok {
		query = "{}"
	}

	err := pyroscopeStart()
	if err != nil {
		return fmt.Errorf("failed to start pyroscope: %w", err)
	}

	benchmark.SetEndHook(DownloadPGO)

	return nil
}

func pyroscopeStart() error {
	tagMap := map[string]string{}
	if config.Host != "" {
		tagMap["hostname"] = config.Host
	}

	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "isucon.go.app",
		ServerAddress:   serverAddr,
		Logger:          slogLogger{},
		Tags:            tagMap,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start pyroscope: %w", err)
	}

	return nil
}

type slogLogger struct{}

func (slogLogger) Infof(format string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	slog.Default().Handler().Handle(context.Background(), r)
}

func (slogLogger) Debugf(format string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprintf(format, args...), pcs[0])
	slog.Default().Handler().Handle(context.Background(), r)
}

func (slogLogger) Errorf(format string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprintf(format, args...), pcs[0])
	slog.Default().Handler().Handle(context.Background(), r)
}

func DownloadPGO(ctx context.Context, b *benchmark.Benchmark) {
	client := querierv1connect.NewQuerierServiceClient(
		http.DefaultClient,
		serverAddr,
		append(
			connectapi.DefaultClientOptions(),
			connect.WithClientOptions(),
		)...,
	)

	res, err := client.SelectMergeSpanProfile(ctx, connect.NewRequest(&querierv1.SelectMergeSpanProfileRequest{
		ProfileTypeID: profileType,
		Start:         b.Start.UnixMilli(),
		End:           b.End.UnixMilli(),
		LabelSelector: query,
	}))
	if err != nil {
		slog.Error("failed to select merge span profile",
			slog.Group("benchmark",
				slog.Time("start", b.Start),
				slog.Time("end", b.End),
				slog.Int64("score", b.Score),
			),
			slog.String("error", err.Error()),
		)
		return
	}

	buf, err := res.Msg.MarshalVT()
	if err != nil {
		slog.Error("failed to marshal vt",
			slog.String("error", err.Error()),
		)
		return
	}

	f, err := os.Create(pgoFile)
	if err != nil {
		slog.Error("failed to create pgo file",
			slog.String("file", pgoFile),
			slog.String("error", err.Error()),
		)
		return
	}
	defer f.Close()

	gzipWriter := gzip.NewWriter(f)
	defer gzipWriter.Close()

	if _, err := io.Copy(gzipWriter, bytes.NewReader(buf)); err != nil {
		slog.Error("failed to copy buffer",
			slog.String("error", err.Error()),
		)
		return
	}
}
