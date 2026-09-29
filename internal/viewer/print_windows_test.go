//go:build windows

package viewer

import (
	"slices"
	"testing"
)

func TestParseLpArgsReadsDialogOptions(t *testing.T) {
	s := printSettings{printer: "Office", pages: "1-3,5", copies: 2, sides: "two-sided-long-edge"}
	got, err := parseLpArgs([]string{"-d", "Office", "-P", "1-3,5", "-n", "2", "-o", "sides=two-sided-long-edge"})
	if err != nil || got != s {
		t.Fatalf("parsed %+v, %v; want %+v", got, err, s)
	}
	if _, err := parseLpArgs([]string{"-o", "fit-to-page"}); err == nil {
		t.Fatal("accepted an option Windows cannot apply")
	}
	if _, err := parseLpArgs([]string{"-d"}); err == nil {
		t.Fatal("accepted -d without a printer")
	}
}

func TestParsePageRanges(t *testing.T) {
	for _, tc := range []struct {
		ranges string
		want   []int
	}{
		{"", []int{0, 1, 2, 3, 4}},
		{"1-3,5", []int{0, 1, 2, 4}},
		{"4-", []int{3, 4}},
		{"-2", []int{0, 1}},
		{" 2 , 2 ", []int{1, 1}},
	} {
		got, err := parsePageRanges(tc.ranges, 5)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("parsePageRanges(%q) = %v, %v; want %v", tc.ranges, got, err, tc.want)
		}
	}
	for _, bad := range []string{"0", "6", "3-2", "a", "1,,2"} {
		if _, err := parsePageRanges(bad, 5); err == nil {
			t.Errorf("parsePageRanges(%q) accepted", bad)
		}
	}
}
