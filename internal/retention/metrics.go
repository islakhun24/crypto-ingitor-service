package retention

import (
	"fmt"
	"sort"
	"strings"
)

type Metrics struct {
	counters map[string]float64
}

func NewMetrics() *Metrics {
	return &Metrics{counters: map[string]float64{}}
}

func (m *Metrics) Observe(result CleanupResult) {
	if m == nil {
		return
	}
	m.add("retention_cleanup_runs_total", map[string]string{"status": result.Status, "dry_run": boolLabel(result.DryRun)}, 1)
	m.add("retention_cleanup_rows_matched_total", nil, float64(result.RowsMatched))
	m.add("retention_cleanup_rows_deleted_total", nil, float64(result.RowsDeleted))
	m.add("retention_cleanup_partitions_dropped_total", nil, float64(result.PartitionsDropped))
	m.add("retention_rollup_rows_read_total", nil, float64(result.RollupRowsRead))
	m.add("retention_rollup_rows_written_total", nil, float64(result.RollupRowsWritten))
	if result.DryRun {
		m.add("retention_cleanup_dry_run_total", nil, 1)
	}
}

func (m *Metrics) Prometheus() string {
	if m == nil || len(m.counters) == 0 {
		return ""
	}

	keys := make([]string, 0, len(m.counters))
	for key := range m.counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(' ')
		builder.WriteString(fmt.Sprintf("%.0f", m.counters[key]))
		builder.WriteByte('\n')
	}

	return builder.String()
}

func (m *Metrics) add(name string, labels map[string]string, value float64) {
	if m.counters == nil {
		m.counters = map[string]float64{}
	}
	key := prometheusKey(name, labels)
	m.counters[key] += value
}

func prometheusKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, strings.ReplaceAll(labels[key], `"`, `\"`)))
	}

	return fmt.Sprintf("%s{%s}", name, strings.Join(parts, ","))
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
