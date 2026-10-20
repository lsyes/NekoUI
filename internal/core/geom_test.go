package core

import "testing"

func TestRectIntersectAndInside(t *testing.T) {
	r := Rect{X: 10, Y: 10, W: 100, H: 50}
	if !r.Inside(Point{X: 10, Y: 10}) || r.Inside(Point{X: 110, Y: 60}) {
		t.Fatal("half-open containment is wrong")
	}
	o := Rect{X: 50, Y: 0, W: 100, H: 100}
	i := r.Intersect(o)
	if i != (Rect{X: 50, Y: 10, W: 60, H: 50}) {
		t.Fatalf("intersect = %+v", i)
	}
	if !r.Intersect(Rect{X: 200, Y: 200, W: 10, H: 10}).IsEmpty() {
		t.Fatal("disjoint rect should be empty")
	}
}

func TestAlignOffset(t *testing.T) {
	if got := AlignOffset(AlignCenter, 100, 40); got != 30 {
		t.Fatalf("center offset = %v", got)
	}
	if got := AlignOffset(AlignEnd, 100, 40); got != 60 {
		t.Fatalf("end offset = %v", got)
	}
}
