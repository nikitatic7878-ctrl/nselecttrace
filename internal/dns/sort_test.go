package dns

import "testing"

func TestSortIsDeterministic(t *testing.T) {
	records := []Record{
		{TypeAAAA, "::1"},
		{TypeA, "192.168.1.1"},
		{TypeAAAA, "fe80::1"},
		{TypeA, "10.0.0.1"},
	}

	first := Sort(records)
	second := Sort(records)

	if len(first) != len(second) {
		t.Fatalf("lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("Sort is not deterministic at %d: %+v vs %+v", i, first[i], second[i])
		}
	}

	// A before AAAA, and lexicographic within a family.
	assertRecords(t, first, []Record{
		{TypeA, "10.0.0.1"},
		{TypeA, "192.168.1.1"},
		{TypeAAAA, "::1"},
		{TypeAAAA, "fe80::1"},
	})
}

func TestSortDoesNotMutateInput(t *testing.T) {
	records := []Record{{TypeAAAA, "::1"}, {TypeA, "10.0.0.1"}}
	before := make([]Record, len(records))
	copy(before, records)

	_ = Sort(records)

	for i := range records {
		if records[i] != before[i] {
			t.Fatalf("Sort mutated its input at %d: %+v, want %+v", i, records[i], before[i])
		}
	}
}

func TestSortKeepsDuplicates(t *testing.T) {
	got := Sort([]Record{{TypeA, "1.2.3.4"}, {TypeA, "1.2.3.4"}})
	if len(got) != 2 {
		t.Errorf("len = %d, want 2 (sorting must not deduplicate)", len(got))
	}
}

func TestSortEmpty(t *testing.T) {
	if got := Sort(nil); len(got) != 0 {
		t.Errorf("Sort(nil) = %v, want empty", got)
	}
	if got := Sort([]Record{}); len(got) != 0 {
		t.Errorf("Sort(empty) = %v, want empty", got)
	}
}

func TestRecordTypeString(t *testing.T) {
	tests := []struct {
		recordType RecordType
		want       string
	}{
		{TypeA, "A"},
		{TypeAAAA, "AAAA"},
		{RecordType(0), "TYPE(?)"},
		{RecordType(99), "TYPE(?)"},
	}
	for _, tt := range tests {
		if got := tt.recordType.String(); got != tt.want {
			t.Errorf("RecordType(%d).String() = %q, want %q", tt.recordType, got, tt.want)
		}
	}
}

func TestResultCounts(t *testing.T) {
	result := Result{Records: []Record{
		{TypeA, "10.0.0.1"},
		{TypeA, "10.0.0.2"},
		{TypeAAAA, "::1"},
	}}

	a, aaaa := result.Counts()
	if a != 2 {
		t.Errorf("A count = %d, want 2", a)
	}
	if aaaa != 1 {
		t.Errorf("AAAA count = %d, want 1", aaaa)
	}

	if empty := (Result{}).Empty(); !empty {
		t.Error("Result{}.Empty() = false, want true")
	}
	if empty := result.Empty(); empty {
		t.Error("a result with records reports Empty() = true")
	}
}
