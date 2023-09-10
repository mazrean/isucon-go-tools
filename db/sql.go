package isudb

import (
	"database/sql"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func DBMetricsSetup[T interface {
	Ping() error
	Close() error
	Query(query string, args ...any) (*sql.Rows, error)
	SetConnMaxIdleTime(d time.Duration)
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}](fn func(string, string) (T, error)) func(string, string) (T, error) {
	return func(driverName string, dataSourceName string) (T, error) {
		var addr string
		switch driverName {
		case "mysql":
			if fixInterpolateParams {
				config, err := mysql.ParseDSN(dataSourceName)
				if err != nil {
					log.Printf("failed to parse dsn: %v\n", err)
					goto CONNECT
				}

				if !config.InterpolateParams {
					config.InterpolateParams = true
					dataSourceName = config.FormatDSN()
				}

				addr = config.Addr
			}

			if isutools.Enable {
				driverName = "isumysql"
			}
		}

	CONNECT:
		var (
			db  T
			err error
		)
		if enableRetry {
			var (
				first = true
				err   error
			)
			for first || err != nil {
				first = false
				db, err = fn(driverName, dataSourceName)
				if err != nil {
					return db, err
				}

				err = db.Ping()
				if err != nil {
					db.Close()
				}
			}
		} else {
			db, err = fn(driverName, dataSourceName)
			if err != nil {
				return db, err
			}

			err = db.Ping()
			if err != nil {
				db.Close()
				return db, err
			}
		}

		db.SetMaxIdleConns(1024)
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(0)

		if isutools.Enable {
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_open_connections",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "idle",
					"addr":   addr,
				},
			}, func() float64 {
				return float64(db.Stats().Idle)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "open",
					"addr":   addr,
				},
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "in_use",
					"addr":   addr,
				},
			}, func() float64 {
				return float64(db.Stats().InUse)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_count",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().WaitCount)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_duration",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().WaitDuration)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_closed",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().MaxOpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_lifetime_closed",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().MaxLifetimeClosed)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_time_closed",
				ConstLabels: map[string]string{
					"addr": addr,
				},
			}, func() float64 {
				return float64(db.Stats().MaxIdleTimeClosed)
			})
		}

		return db, err
	}
}

type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type RegexpPair struct {
	target string
	subs   string
}

var regexpPairs = []RegexpPair{
	{
		target: `[+\-]{0,1}\b\d+\b`,
		subs:   "N",
	}, {
		target: `\b0x[0-9A-Fa-f]+\b`,
		subs:   "0xN",
	}, {
		target: `'[^']+'`,
		subs:   "S",
	}, {
		target: `"[^"]+"`,
		subs:   "S",
	}, {
		target: `(([NS]\s*,\s*){4,})`,
		subs:   "...",
	},
}

var regexpNormalizers []*RegexpNormalizer

type RegexpNormalizer struct {
	re   *regexp.Regexp
	subs string
}

func (p *RegexpNormalizer) Normalize(q string) string {
	return p.re.ReplaceAllString(q, p.subs)
}

func Normalize(queries []string) []string {
	normalized := make([]string, 0, len(queries))
	for _, query := range queries {
		for _, trimString := range trimStrings {
			query = strings.Trim(query, trimString)
		}
		query = strings.TrimFunc(query, (&Trimer{}).TrimFunc)

		for _, regexpNormalizer := range regexpNormalizers {
			query = regexpNormalizer.Normalize(query)
		}

		normalized = append(normalized, query)
	}

	return normalized
}

var trimStrings = []string{`\'`, `\"`}

type Trimer struct {
	lastChar rune
}

func (tr *Trimer) TrimFunc(c rune) bool {
	if tr.lastChar == ' ' && c == ' ' {
		return true
	}

	if !utf8.ValidRune(c) {
		return true
	}

	tr.lastChar = c
	return false
}
