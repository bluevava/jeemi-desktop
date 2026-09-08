package processmemory

import (
	"math"
	"testing"
)

func TestCombineCountsHelperOnceAndUsesCoreFromIPC(t *testing.T) {
	desktop := buildSnapshot(valuePointer(10), valuePointer(20), valuePointer(999))
	helper := HelperSnapshot{HelperBytes: valuePointer(30), MihomoBytes: valuePointer(40), MihomoPID: 71, WebViewBytes: valuePointer(999)}
	result := Combine(desktop, helper, 71)
	if *result.ClientBytes != 40 || *result.WebViewBytes != 20 || *result.MihomoBytes != 40 || *result.TotalBytes != 100 {
		t.Fatalf("unexpected aggregate: %+v", result)
	}
}

func TestCombineRejectsMissingAndMismatchedHelperMeasurements(t *testing.T) {
	desktop := buildSnapshot(valuePointer(10), valuePointer(20), valuePointer(100))
	for _, helper := range []HelperSnapshot{
		{},
		{HelperBytes: valuePointer(30), MihomoBytes: valuePointer(40), MihomoPID: 72},
		{HelperBytes: valuePointer(30), MihomoPID: 71},
	} {
		result := Combine(desktop, helper, 71)
		if result.MihomoBytes != nil || result.TotalBytes != nil {
			t.Fatal("stale or missing core produced a total")
		}
	}
	if Combine(desktop, HelperSnapshot{MihomoBytes: valuePointer(40), MihomoPID: 71}, 71).ClientBytes != nil {
		t.Fatal("missing helper memory was counted as zero")
	}
}

func TestCombineUsesScopedWebFallbackAndDistinguishesAbsence(t *testing.T) {
	result := Combine(buildSnapshot(valuePointer(10), nil, nil), HelperSnapshot{HelperBytes: valuePointer(5), MihomoBytes: valuePointer(0), WebViewBytes: valuePointer(20)}, 0)
	if result.TotalBytes == nil || *result.TotalBytes != 35 {
		t.Fatal("web fallback not included")
	}
	result = Combine(buildSnapshot(valuePointer(10), valuePointer(20), nil), AbsentHelper(), 0)
	if result.TotalBytes == nil || *result.TotalBytes != 30 {
		t.Fatal("confirmed absence should contribute zero")
	}
	if Combine(buildSnapshot(valuePointer(10), valuePointer(20), nil), HelperSnapshot{}, 0).TotalBytes != nil {
		t.Fatal("unreachable helper reported as absent")
	}
}

func TestMemoryAdditionDoesNotOverflow(t *testing.T) {
	if sum(valuePointer(math.MaxUint64), valuePointer(1)) != nil {
		t.Fatal("overflowed memory sum")
	}
	if buildSnapshot(valuePointer(math.MaxUint64), valuePointer(1), valuePointer(0)).TotalBytes != nil {
		t.Fatal("overflowed total")
	}
}
