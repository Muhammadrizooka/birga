package domain

import "testing"

func TestIsValidChildAgeMonths(t *testing.T) {
	for _, m := range []int{1, 2, 10, 11, 12, 18, 30, 84, 96, 144} {
		if !IsValidChildAgeMonths(m) {
			t.Errorf("%d months should be valid", m)
		}
	}

	for _, m := range []int{-1, 0, 13, 20, 90, 102, 156} {
		if IsValidChildAgeMonths(m) {
			t.Errorf("%d months should be invalid", m)
		}
	}
}

func TestChildAgeDB(t *testing.T) {
	for _, m := range []int{1, 2, 10, 12, 18, 30, 84, 96, 144} {
		if got := ChildAgeFromDB(ChildAgeToDB(m)); got != m {
			t.Errorf("%d months came back as %d", m, got)
		}
	}

	if ChildAgeToDB(18) != 1.5 || ChildAgeFromDB(4) != 48 {
		t.Fatal("unexpected conversion")
	}

	if (Child{AgeMonths: 54}).AgeYears() != 4 {
		t.Fatal("54 months is 4 whole years")
	}
}
