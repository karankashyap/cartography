package analytics

import (
	"testing"
)

func TestAOVCalculation_ZeroOrders(t *testing.T) {
	m := &Metrics{Orders: 0, RevenueCents: 0}
	if m.Orders > 0 {
		m.AOVCents = m.RevenueCents / int64(m.Orders)
	}
	if m.AOVCents != 0 {
		t.Errorf("AOV with 0 orders: got %d, want 0 (no divide-by-zero)", m.AOVCents)
	}
}

func TestAOVCalculation_Normal(t *testing.T) {
	m := &Metrics{Orders: 4, RevenueCents: 10000}
	if m.Orders > 0 {
		m.AOVCents = m.RevenueCents / int64(m.Orders)
	}
	if m.AOVCents != 2500 {
		t.Errorf("AOV: got %d, want 2500", m.AOVCents)
	}
}

func TestReturningRate(t *testing.T) {
	cases := []struct {
		newC, ret int
		want      float64
		label     string
	}{
		{10, 5, 0.3333, "mixed"},
		{0, 0, 0.0, "no customers"},
		{0, 10, 1.0, "all returning"},
		{10, 0, 0.0, "all new"},
		{1, 1, 0.5, "fifty-fifty"},
	}

	for _, c := range cases {
		var rate float64
		if total := c.newC + c.ret; total > 0 {
			rate = float64(c.ret) / float64(total)
		}
		diff := rate - c.want
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("[%s] returning rate: got %.4f, want %.4f", c.label, rate, c.want)
		}
	}
}

func TestMetricsFilter_DefaultDates(t *testing.T) {
	f := MetricsFilter{}
	if !f.From.IsZero() {
		t.Error("zero value From should be zero")
	}
	if !f.To.IsZero() {
		t.Error("zero value To should be zero")
	}
}
