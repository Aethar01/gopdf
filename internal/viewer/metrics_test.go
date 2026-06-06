package viewer

import (
	"reflect"
	"testing"
)

func TestMetricPageOrderPrioritizesNearbyPages(t *testing.T) {
	got := metricPageOrder(8, 3)
	want := []int{4, 2, 5, 1, 6, 0, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metricPageOrder(8, 3) = %v, want %v", got, want)
	}
}

func TestMetricPageOrderClampsStartPage(t *testing.T) {
	got := metricPageOrder(4, 99)
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metricPageOrder(4, 99) = %v, want %v", got, want)
	}
}

func TestMetricPageOrderSinglePage(t *testing.T) {
	if got := metricPageOrder(1, 0); got != nil {
		t.Fatalf("metricPageOrder(1, 0) = %v, want nil", got)
	}
}
