//go:build linux
// +build linux

package isutools

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/procfs"
	"golang.org/x/sync/singleflight"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "procfs"
	defaultProcFS       = "/proc"
	statTTL             = time.Second
	procUpdateRate      = 2 * time.Second
)

func init() {
	if !Enable {
		return
	}

	procFSPath, ok := os.LookupEnv("ISUTOOLS_PROCFS")
	if !ok {
		procFSPath = defaultProcFS
	}

	procFS, err := procfs.NewFS(procFSPath)
	if err != nil {
		log.Printf("failed to init procfs: %v", err)
		return
	}

	cpuInfos, err := procFS.CPUInfo()
	if err != nil {
		log.Printf("failed to get cpuinfo: %v", err)
		return
	}

	if len(cpuInfos) == 0 {
		log.Printf("cpuinfo is empty")
		return
	}

	hertz := cpuInfos[0].CPUMHz
	hertsGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "cpu_hertz",
	})
	hertsGauge.Set(float64(hertz))

	go updateProcListWorker(procFS)
}

func updateProcListWorker(procFS procfs.FS) {
	ticker := time.NewTicker(2 * time.Second)
	procMap := map[int]*procMetrics{}
	for range ticker.C {
		procs, err := procFS.AllProcs()
		if err != nil {
			log.Printf("failed to get all procs: %v", err)
			continue
		}

		activeProcMap := make(map[int]struct{}, len(procs))
		for _, proc := range procs {
			if proc.PID <= 0 {
				continue
			}

			activeProcMap[proc.PID] = struct{}{}

			if _, ok := procMap[proc.PID]; !ok {
				pm, err := newProcMetrics(proc)
				if err != nil {
					log.Printf("failed to new proc metrics: %v", err)
					continue
				}
				procMap[proc.PID] = pm
			}
		}

		for pid, pm := range procMap {
			if _, ok := activeProcMap[pid]; !ok {
				pm.unregister()
				delete(procMap, pid)
			}
		}
	}
}

var sf = &singleflight.Group{}

type procMetrics struct {
	proc          procfs.Proc
	stat          procfs.ProcStat
	statExpiresAt time.Time
	collectors    []prometheus.Collector
}

func newProcMetrics(proc procfs.Proc) (*procMetrics, error) {
	stat, err := proc.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get process stat: %w", err)
	}

	pm := &procMetrics{
		proc:          proc,
		stat:          stat,
		statExpiresAt: time.Now().Add(statTTL),
	}

	labels := map[string]string{
		"cmd": stat.Comm,
		"pid": strconv.Itoa(proc.PID),
	}

	userCPUGauge := promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   prometheusNamespace,
		Subsystem:   prometheusSubsystem,
		Name:        "user_cpu_clock_count",
		ConstLabels: labels,
	}, pm.userCPUFunc)
	pm.collectors = append(pm.collectors, userCPUGauge)

	systemCPUGauge := promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   prometheusNamespace,
		Subsystem:   prometheusSubsystem,
		Name:        "system_cpu_clock_count",
		ConstLabels: labels,
	}, pm.systemCPUFunc)
	pm.collectors = append(pm.collectors, systemCPUGauge)

	memoryGauge := promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   prometheusNamespace,
		Subsystem:   prometheusSubsystem,
		Name:        "memory",
		ConstLabels: labels,
	}, pm.memoryFunc)
	pm.collectors = append(pm.collectors, memoryGauge)

	return pm, nil
}

func (pm *procMetrics) unregister() {
	for _, c := range pm.collectors {
		prometheus.Unregister(c)
	}
}

func (pm *procMetrics) getStat() (procfs.ProcStat, error) {
	iStat, err, _ := sf.Do(strconv.Itoa(pm.proc.PID), func() (interface{}, error) {
		if pm.statExpiresAt.After(time.Now()) {
			return pm.stat, nil
		}

		newStat, err := pm.proc.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to get process stat: %w", err)
		}

		pm.stat = newStat
		pm.statExpiresAt = time.Now().Add(statTTL)

		return pm.stat, nil
	})
	if err != nil {
		return procfs.ProcStat{}, err
	}

	stat, ok := iStat.(procfs.ProcStat)
	if !ok {
		return procfs.ProcStat{}, fmt.Errorf("failed to cast stat")
	}

	return stat, nil
}

func (pm *procMetrics) userCPUFunc() float64 {
	stat, err := pm.getStat()
	if err != nil {
		return 0
	}

	return float64(stat.UTime)
}

func (pm *procMetrics) systemCPUFunc() float64 {
	stat, err := pm.getStat()
	if err != nil {
		return 0
	}

	return float64(stat.STime)
}

func (pm *procMetrics) memoryFunc() float64 {
	stat, err := pm.getStat()
	if err != nil {
		return 0
	}

	return float64(stat.ResidentMemory())
}
