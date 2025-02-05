package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type CacheEntry struct {
	Data       []byte
	ExpiryTime time.Time
}

type WeatherCache struct {
	sync.RWMutex
	entries map[string]CacheEntry
	expiry  time.Duration
}

func NewWeatherCache(apiKey string, expiry time.Duration) *WeatherCache {
	return &WeatherCache{
		entries: make(map[string]CacheEntry),
		expiry:  expiry,
	}
}

func (c *WeatherCache) Get(city string) ([]byte, bool) {
	c.RLock()
	defer c.RUnlock()
	entry, exists := c.entries[city]
	if !exists || time.Now().After(entry.ExpiryTime) {
		return nil, false
	}
	return entry.Data, true
}

func (c *WeatherCache) Set(city string, data []byte) {
	c.Lock()
	defer c.Unlock()
	c.entries[city] = CacheEntry{
		Data:       data,
		ExpiryTime: time.Now().Add(c.expiry),
	}
}

func main() {
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENWEATHER_API_KEY environment variable not set")
	}

	cache := NewWeatherCache(apiKey, 5*time.Minute)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/api/weather", func(w http.ResponseWriter, r *http.Request) {
		city := r.URL.Query().Get("city")
		if city == "" {
			http.Error(w, "City parameter is required", http.StatusBadRequest)
			return
		}

		cachedData, found := cache.Get(city)
		if found {
			w.Header().Set("Content-Type", "application/json")
			w.Write(cachedData)
			return
		}

		url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)
		resp, err := http.Get(url)
		if err != nil {
			http.Error(w, "Failed to fetch weather data", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "City not found", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read response", http.StatusInternalServerError)
			return
		}

		cache.Set(city, body)

		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
