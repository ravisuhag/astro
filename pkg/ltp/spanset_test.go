package ltp

// Characterization tests for spanSet, an unexported type used by both session
// machines, so they live in the ltp package itself rather than ltp_test.
// Written before the in-place rewrite of add (Step 3) to pin its behaviour:
// sorted, merged and non-overlapping, regardless of insertion order.

import "testing"

// list is a small helper to make a spanSet's contents comparable in test
// failure messages.
func (s *spanSet) list() []span {
	return append([]span(nil), s.spans...)
}

func spansEqual(a, b []span) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSpanSetAddOverlap(t *testing.T) {
	var s spanSet
	s.add(5, 15)
	s.add(10, 20)
	want := []span{{5, 20}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddAdjacency(t *testing.T) {
	var s spanSet
	s.add(5, 10)
	s.add(20, 30)
	s.add(10, 20) // touches both neighbours exactly, bridging them
	want := []span{{5, 30}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddExactDuplicate(t *testing.T) {
	var s spanSet
	s.add(5, 10)
	s.add(5, 10)
	want := []span{{5, 10}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddFullContainment(t *testing.T) {
	var s spanSet
	s.add(0, 100)
	s.add(10, 20) // wholly inside the existing span
	want := []span{{0, 100}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddReverseOrder(t *testing.T) {
	var s spanSet
	s.add(20, 30)
	s.add(10, 15)
	s.add(0, 5)
	want := []span{{0, 5}, {10, 15}, {20, 30}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddNoOpWhenEndNotAfterStart(t *testing.T) {
	var s spanSet
	s.add(5, 10)
	s.add(10, 10) // end == start
	s.add(10, 5)  // end < start
	want := []span{{5, 10}}
	if got := s.list(); !spansEqual(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestSpanSetAddManyNonAdjacentSpansStaySorted(t *testing.T) {
	// Characterizes the case the O(N^2) rebuild was slow on: many
	// non-adjacent ranges inserted in increasing order.
	var s spanSet
	for i := uint64(0); i < 200; i++ {
		start := i * 10
		s.add(start, start+5)
	}
	if len(s.spans) != 200 {
		t.Fatalf("got %d spans, want 200", len(s.spans))
	}
	for i, sp := range s.spans {
		wantStart := uint64(i) * 10
		if sp.start != wantStart || sp.end != wantStart+5 {
			t.Fatalf("span[%d] = %v, want {%d %d}", i, sp, wantStart, wantStart+5)
		}
	}
}
