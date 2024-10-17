package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	Reset  = "\033[0m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Blue   = "\033[34m"
)

type Location struct {
	Name        string
	Coordinates string
}

type Weather struct {
	Temp float64 `json:"temp"`
}

type Place struct {
	Name        string `json:"name"`
	Xid         string `json:"xid"`
	Description string
}

type PlacesResponse struct {
	Features []struct {
		Properties Place `json:"properties"`
	} `json:"features"`
}

func getCoords(locationName, apiKey string) ([]Location, error) {
	url := fmt.Sprintf("https://graphhopper.com/api/1/geocode?q=%s&key=%s&locale=ru", locationName, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Hits []struct {
			Point struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"point"`
			Name string `json:"name"`
		} `json:"hits"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Hits) == 0 {
		return nil, fmt.Errorf("location not found")
	}

	var locations []Location
	for _, hit := range result.Hits {
		coordinates := fmt.Sprintf("%f,%f", hit.Point.Lat, hit.Point.Lng)
		locations = append(locations, Location{Name: hit.Name, Coordinates: coordinates})
	}
	return locations, nil
}

func getWeather(coordinates, apiKey string) (*Weather, error) {
	coords := strings.Split(coordinates, ",")
	if len(coords) != 2 {
		return nil, fmt.Errorf("invalid coordinates format")
	}

	lat := coords[0]
	lon := coords[1]

	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%s&lon=%s&appid=%s&units=metric", lat, lon, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Main Weather `json:"main"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Main, nil
}

func getPlaces(coordinates, apiKey string) ([]Place, error) {
	coords := strings.Split(coordinates, ",")
	if len(coords) != 2 {
		return nil, fmt.Errorf("invalid coordinates format")
	}

	lat := coords[0]
	lon := coords[1]

	radius := 1000
	limit := 3

	url := fmt.Sprintf("https://api.opentripmap.com/0.1/en/places/radius?radius=%d&lon=%s&lat=%s&apikey=%s&limit=%d", radius, lon, lat, apiKey, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var placesResponse PlacesResponse
	if err := json.Unmarshal(body, &placesResponse); err != nil {
		return nil, err
	}

	var places []Place
	for _, feature := range placesResponse.Features {
		places = append(places, feature.Properties)
	}

	return places, nil
}

func getPlaceDescription(xid, apiKey string) (string, error) {
	url := fmt.Sprintf("https://api.opentripmap.com/0.1/en/places/xid/%s?apikey=%s", xid, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var description struct {
		Extracts struct {
			Text string `json:"text"`
		} `json:"wikipedia_extracts"`
	}

	if err := json.Unmarshal(body, &description); err != nil {
		return "", err
	}

	return description.Extracts.Text, nil
}

func main() {
	locationName := "Академгородок"
	graphHopperAPIKey := "4b96191d-c3a8-4fc7-8bf8-f0878968a8fa"
	openWeatherMapAPIKey := "fc9328160590d0fdc8285164b116b415"
	openTripMapAPIKey := "5ae2e3f221c38a28845f05b62b3686bd0625c01d9b503956e45ddc40"

	locations, err := getCoords(locationName, graphHopperAPIKey)
	if err != nil {
		log.Fatal("error with location search:", err)
	}

	fmt.Println("found locations:")
	for i, loc := range locations {
		fmt.Printf("%d: "+Green+"%s "+Reset+"(%s)\n", i, loc.Name, loc.Coordinates)
	}

	var choice int
	fmt.Print("choose a location by number: ")
	fmt.Scan(&choice)

	selectedLocation := locations[choice]

	var wg sync.WaitGroup

	weatherChan := make(chan *Weather)
	placesChan := make(chan []Place)
	wg.Add(2)

	go func() {
		defer wg.Done()
		weather, err := getWeather(selectedLocation.Coordinates, openWeatherMapAPIKey)
		if err != nil {
			log.Println("error in getting weather:", err)
			return
		}
		weatherChan <- weather
	}()

	go func() {
		defer wg.Done()
		places, err := getPlaces(selectedLocation.Coordinates, openTripMapAPIKey)
		if err != nil {
			log.Println("error in getting places:", err)
			return
		}

		wg.Add(len(places))

		for i := range places {
			go func(p *Place) {
				defer wg.Done()
				description, err := getPlaceDescription(p.Xid, openTripMapAPIKey)
				if err != nil {
					log.Println("error in getting description for place:", p.Name, err)
				} else {
					p.Description = description
				}
			}(&places[i])
		}

		placesChan <- places
	}()

	go func() {
		wg.Wait()
		close(weatherChan)
		close(placesChan)
	}()

	weather := <-weatherChan
	places := <-placesChan

	fmt.Printf("Location: "+Green+"%s\n"+Reset+"Coords:"+Yellow+" %s\n"+Reset, selectedLocation.Name, selectedLocation.Coordinates)
	fmt.Printf("Temp:"+Blue+" %.2f°C\n"+Reset, weather.Temp)
	fmt.Println("Interesting places:")

	for _, place := range places {
		fmt.Printf(Green+" - %s\n"+Reset, place.Name)
		fmt.Printf("   Description: %s\n", place.Description)
	}
}
