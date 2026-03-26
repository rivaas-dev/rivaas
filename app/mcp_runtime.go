// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

const (
	goroutineWarningThreshold = 1000
	goroutineHighThreshold    = 5000
	heapWarningMB             = 512
	heapHighMB                = 1024
	gcPauseWarningMS          = 10
)

type runtimeStatsResponse struct {
	Service    string             `json:"service"`
	Version    string             `json:"version"`
	Uptime     string             `json:"uptime"`
	UptimeSecs float64            `json:"uptime_secs"`
	Goroutines int                `json:"goroutines"`
	Memory     runtimeMemoryStats `json:"memory"`
	GoVersion  string             `json:"go_version"`
	NumCPU     int                `json:"num_cpu"`
	GOMAXPROCS int                `json:"gomaxprocs"`
	Signals    []string           `json:"signals"`
}

type runtimeMemoryStats struct {
	HeapAllocBytes  uint64  `json:"heap_alloc_bytes"`
	HeapAllocMB     float64 `json:"heap_alloc_mb"`
	HeapSysBytes    uint64  `json:"heap_sys_bytes"`
	HeapObjects     uint64  `json:"heap_objects"`
	StackInuseBytes uint64  `json:"stack_inuse_bytes"`
	TotalAllocBytes uint64  `json:"total_alloc_bytes"`
	SysBytes        uint64  `json:"sys_bytes"`
	Mallocs         uint64  `json:"mallocs"`
	Frees           uint64  `json:"frees"`
	GCCycles        uint32  `json:"gc_cycles"`
	LastGC          string  `json:"last_gc"`
	GCPauseTotalNs  uint64  `json:"gc_pause_total_ns"`
	NextGCBytes     uint64  `json:"next_gc_bytes"`
}

type goroutineProfileResponse struct {
	TotalGoroutines int              `json:"total_goroutines"`
	StateSummary    map[string]int   `json:"state_summary"`
	Goroutines      []goroutineEntry `json:"goroutines"`
	Signals         []string         `json:"signals"`
}

type goroutineEntry struct {
	Header string `json:"header"`
	State  string `json:"state"`
	Stack  string `json:"stack"`
}

type gcStatsResponse struct {
	GCCycles       uint32   `json:"gc_cycles"`
	LastGC         string   `json:"last_gc"`
	PauseTotalNs   uint64   `json:"pause_total_ns"`
	PauseTotalMs   float64  `json:"pause_total_ms"`
	LastPauseNs    uint64   `json:"last_pause_ns"`
	LastPauseMs    float64  `json:"last_pause_ms"`
	AvgPauseMs     float64  `json:"avg_pause_ms"`
	NextGCBytes    uint64   `json:"next_gc_bytes"`
	GCCPUFraction  float64  `json:"gc_cpu_fraction"`
	EnableGC       bool     `json:"enable_gc"`
	HeapAllocBytes uint64   `json:"heap_alloc_bytes"`
	HeapObjects    uint64   `json:"heap_objects"`
	Mallocs        uint64   `json:"mallocs"`
	Frees          uint64   `json:"frees"`
	LiveObjects    uint64   `json:"live_objects"`
	Signals        []string `json:"signals"`
}

type buildInfoResponse struct {
	Available    bool              `json:"available"`
	GoVersion    string            `json:"go_version,omitempty"`
	Path         string            `json:"path,omitempty"`
	MainModule   string            `json:"main_module,omitempty"`
	MainVersion  string            `json:"main_version,omitempty"`
	Dependencies []buildDep        `json:"dependencies,omitempty"`
	Settings     map[string]string `json:"settings,omitempty"`
	Signals      []string          `json:"signals"`
}

type buildDep struct {
	Path            string `json:"path"`
	Version         string `json:"version"`
	ReplacedBy      string `json:"replaced_by,omitempty"`
	ReplacedVersion string `json:"replaced_version,omitempty"`
}

func collectRuntimeStats(serviceName, serviceVersion string, start time.Time) *runtimeStatsResponse {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	numGoroutines := runtime.NumGoroutine()
	heapMB := float64(m.HeapAlloc) / (1024 * 1024)
	uptime := time.Since(start)

	var signals []string

	if numGoroutines > goroutineHighThreshold {
		signals = append(signals, fmt.Sprintf("goroutine count (%d) is very high — likely leak or unbounded concurrency", numGoroutines))
	} else if numGoroutines > goroutineWarningThreshold {
		signals = append(signals, fmt.Sprintf("goroutine count (%d) exceeds typical threshold — investigate if expected", numGoroutines))
	}

	if heapMB > heapHighMB {
		signals = append(signals, fmt.Sprintf("heap usage (%.1f MB) is very high — check for memory leaks", heapMB))
	} else if heapMB > heapWarningMB {
		signals = append(signals, fmt.Sprintf("heap usage (%.1f MB) is elevated — monitor for growth trends", heapMB))
	}

	return &runtimeStatsResponse{
		Service:    serviceName,
		Version:    serviceVersion,
		Uptime:     uptime.String(),
		UptimeSecs: uptime.Seconds(),
		Goroutines: numGoroutines,
		Memory: runtimeMemoryStats{
			HeapAllocBytes:  m.HeapAlloc,
			HeapAllocMB:     heapMB,
			HeapSysBytes:    m.HeapSys,
			HeapObjects:     m.HeapObjects,
			StackInuseBytes: m.StackInuse,
			TotalAllocBytes: m.TotalAlloc,
			SysBytes:        m.Sys,
			Mallocs:         m.Mallocs,
			Frees:           m.Frees,
			GCCycles:        m.NumGC,
			LastGC:          time.Unix(0, safeInt64(m.LastGC)).Format(time.RFC3339),
			GCPauseTotalNs:  m.PauseTotalNs,
			NextGCBytes:     m.NextGC,
		},
		GoVersion:  runtime.Version(),
		NumCPU:     runtime.NumCPU(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		Signals:    signals,
	}
}

func collectGoroutineProfile() *goroutineProfileResponse {
	buf := make([]byte, 1<<20) //nolint:makezero // runtime.Stack requires a pre-sized buffer
	n := runtime.Stack(buf, true)
	stackDump := string(buf[:n])

	goroutines := parseGoroutineStacks(stackDump)
	numGoroutines := runtime.NumGoroutine()

	var signals []string
	if numGoroutines > goroutineHighThreshold {
		signals = append(signals, fmt.Sprintf("goroutine count (%d) is very high — likely leak or unbounded concurrency", numGoroutines))
	} else if numGoroutines > goroutineWarningThreshold {
		signals = append(signals, fmt.Sprintf("goroutine count (%d) exceeds typical threshold", numGoroutines))
	}

	stateCounts := make(map[string]int)
	for _, g := range goroutines {
		stateCounts[g.State]++
	}

	return &goroutineProfileResponse{
		TotalGoroutines: numGoroutines,
		StateSummary:    stateCounts,
		Goroutines:      goroutines,
		Signals:         signals,
	}
}

func parseGoroutineStacks(dump string) []goroutineEntry {
	sections := strings.Split(dump, "\n\n")
	var goroutines []goroutineEntry

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		lines := strings.SplitN(section, "\n", 2)
		header := lines[0]

		var state string
		if start := strings.Index(header, "["); start != -1 {
			if end := strings.Index(header[start:], "]"); end != -1 {
				state = header[start+1 : start+end]
			}
		}

		stack := ""
		if len(lines) > 1 {
			stack = lines[1]
		}

		goroutines = append(goroutines, goroutineEntry{
			Header: header,
			State:  state,
			Stack:  stack,
		})
	}

	return goroutines
}

func collectGCStats() *gcStatsResponse {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var signals []string

	lastPauseNs := uint64(0)
	if m.NumGC > 0 {
		lastPauseNs = m.PauseNs[(m.NumGC+255)%256]
	}
	lastPauseMS := float64(lastPauseNs) / 1e6

	if lastPauseMS > gcPauseWarningMS {
		signals = append(signals, fmt.Sprintf("last GC pause (%.2f ms) is high — may cause latency spikes", lastPauseMS))
	}

	avgPauseMS := float64(0)
	if m.NumGC > 0 {
		avgPauseMS = float64(m.PauseTotalNs) / float64(m.NumGC) / 1e6
	}

	return &gcStatsResponse{
		GCCycles:       m.NumGC,
		LastGC:         time.Unix(0, safeInt64(m.LastGC)).Format(time.RFC3339),
		PauseTotalNs:   m.PauseTotalNs,
		PauseTotalMs:   float64(m.PauseTotalNs) / 1e6,
		LastPauseNs:    lastPauseNs,
		LastPauseMs:    lastPauseMS,
		AvgPauseMs:     avgPauseMS,
		NextGCBytes:    m.NextGC,
		GCCPUFraction:  m.GCCPUFraction,
		EnableGC:       m.EnableGC,
		HeapAllocBytes: m.HeapAlloc,
		HeapObjects:    m.HeapObjects,
		Mallocs:        m.Mallocs,
		Frees:          m.Frees,
		LiveObjects:    m.Mallocs - m.Frees,
		Signals:        signals,
	}
}

func collectBuildInfo() *buildInfoResponse {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return &buildInfoResponse{
			Available: false,
			Signals:   []string{"build info not available — binary may not have been built with module support"},
		}
	}

	var deps []buildDep
	for _, dep := range bi.Deps {
		d := buildDep{
			Path:    dep.Path,
			Version: dep.Version,
		}
		if dep.Replace != nil {
			d.ReplacedBy = dep.Replace.Path
			d.ReplacedVersion = dep.Replace.Version
		}
		deps = append(deps, d)
	}

	settings := make(map[string]string)
	for _, s := range bi.Settings {
		settings[s.Key] = s.Value
	}

	return &buildInfoResponse{
		Available:    true,
		GoVersion:    bi.GoVersion,
		Path:         bi.Path,
		MainModule:   bi.Main.Path,
		MainVersion:  bi.Main.Version,
		Dependencies: deps,
		Settings:     settings,
		Signals:      []string{},
	}
}

// safeInt64 converts a uint64 to int64, clamping at math.MaxInt64 to avoid overflow.
func safeInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v) //nolint:gosec // overflow guarded by the check above
}
