package config

import (
	"reflect"
	"testing"
)

func TestParseCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"main", []string{"main"}},
		{"main,master", []string{"main", "master"}},
		{" main , master ", []string{"main", "master"}},
		{",,main,,", []string{"main"}},
	}
	for _, tc := range cases {
		got := parseCSV(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseCSV(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		err  bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"1", true, false},
		{"0", false, false},
		{"YES", true, false},
		{"on", true, false},
		{"off", false, false},
		{"", false, false},
		{"banana", false, true},
	}
	for _, tc := range cases {
		got, err := parseBool(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("parseBool(%q) err=%v want err=%v", tc.in, err, tc.err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseBool(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsKnown(t *testing.T) {
	for _, k := range KnownKeys() {
		if !IsKnown(k) {
			t.Errorf("IsKnown(%q) = false", k)
		}
	}
	if IsKnown("wtguard.notARealKey") {
		t.Errorf("IsKnown(unknown) = true")
	}
}
