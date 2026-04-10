package main

import "testing"

func TestDegreesToCardinal(t *testing.T) {
	cases := []struct {
		deg  float64
		want string
	}{
		{0, "N"},
		{360, "N"},  // wraps
		{22.4, "N"}, // just below NE threshold
		{22.5, "NE"},
		{45, "NE"},
		{90, "E"},
		{135, "SE"},
		{180, "S"},
		{225, "SW"},
		{270, "W"},
		{315, "NW"},
		{337.4, "NW"},
		{337.5, "N"}, // rounds back to N
	}
	for _, c := range cases {
		got := degreesToCardinal(c.deg)
		if got != c.want {
			t.Errorf("degreesToCardinal(%v) = %q, want %q", c.deg, got, c.want)
		}
	}
}

func TestWmoDescription(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear"},
		{1, "Mostly Clear"},
		{2, "Partly Cloudy"},
		{3, "Overcast"},
		{45, "Foggy"},
		{61, "Light Rain"},
		{63, "Rain"},
		{65, "Heavy Rain"},
		{71, "Light Snow"},
		{80, "Showers"},
		{95, "Thunderstorm"},
		{99, "Unknown"}, // undefined code
		{-1, "Unknown"},
	}
	for _, c := range cases {
		got := wmoDescription(c.code)
		if got != c.want {
			t.Errorf("wmoDescription(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}
