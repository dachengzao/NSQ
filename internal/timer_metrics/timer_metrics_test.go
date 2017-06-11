package timer_metrics

import (
	"log"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	m := NewTimerMetrics(5, "prefix")
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11} {
		m.StatusDuration(time.Duration(n) * time.Second)
		stats := m.Stats()
		log.Printf("%s", stats)
		log.Printf("> %#v", m.timings)
	}
	stats := m.Stats()
	if stats.Avg != time.Duration(9)*time.Second {
		t.Errorf("avg is %s expected 9s", stats.Avg)
	}
	if stats.P99 != time.Duration(11)*time.Second {
		t.Errorf("99th is %s expected 11s", stats.P99)
	}
}
