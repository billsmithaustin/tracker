package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

type weatherResult struct {
	TempF     float64
	WindMph   float64
	WindDir   string
	Condition string
}

func fetchWeather(lat, lng float64) *weatherResult {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
			"&current=temperature_2m,wind_speed_10m,wind_direction_10m,weather_code"+
			"&temperature_unit=fahrenheit&wind_speed_unit=mph&timezone=auto",
		lat, lng,
	)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var payload struct {
		Current struct {
			Temperature2m    float64 `json:"temperature_2m"`
			WindSpeed10m     float64 `json:"wind_speed_10m"`
			WindDirection10m float64 `json:"wind_direction_10m"`
			WeatherCode      int     `json:"weather_code"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}

	c := payload.Current
	return &weatherResult{
		TempF:     math.Round(c.Temperature2m),
		WindMph:   math.Round(c.WindSpeed10m),
		WindDir:   degreesToCardinal(c.WindDirection10m),
		Condition: wmoDescription(c.WeatherCode),
	}
}

func degreesToCardinal(deg float64) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	return dirs[int(math.Round(deg/45))%8]
}

func wmoDescription(code int) string {
	descriptions := map[int]string{
		0: "Clear", 1: "Mostly Clear", 2: "Partly Cloudy", 3: "Overcast",
		45: "Foggy", 48: "Icy Fog",
		51: "Light Drizzle", 53: "Drizzle", 55: "Heavy Drizzle",
		61: "Light Rain", 63: "Rain", 65: "Heavy Rain",
		71: "Light Snow", 73: "Snow", 75: "Heavy Snow",
		80: "Showers", 81: "Heavy Showers", 82: "Violent Showers",
		95: "Thunderstorm", 96: "Thunderstorm w/ Hail",
	}
	if d, ok := descriptions[code]; ok {
		return d
	}
	return "Unknown"
}
