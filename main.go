package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"googlemaps.github.io/maps"
)

type BiteBody struct {
	Verb      string  `json:"verb"`
	Long      float64 `json:"long"`
	Lat       float64 `json:"lat"`
	Radius    uint    `json:"radius"`
	MinPrice  int     `json:"minPrice"`
	MaxPrice  int     `json:"maxPrice"`
	PageToken string  `json:"pageToken"`
	PhotoRef  string  `json:"photoRef"`
}

var errorLogger = log.New(os.Stderr, "ERROR ", log.Llongfile)
var apiKey = os.Getenv("API_KEY")

func check(err error) {
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}
}

func main() {
	lambda.Start(router)
}

func router(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Top")
	switch req.HTTPMethod {
	case "POST":
		log.Println("router caught")
		return handleRequest(req)
	default:
		log.Println("router failed")
		log.Printf("%s", req.HTTPMethod)
		return clientError(http.StatusMethodNotAllowed)
	}
}

func handleRequest(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var parameters BiteBody
	body := req.Body
	json.Unmarshal([]byte(body), &parameters)
	verb := parameters.Verb
	log.Printf("Verb is %s", verb)
	if verb == "create" {
		return handleCreate(parameters.Lat, parameters.Long, parameters.Radius, parameters.MinPrice, parameters.MaxPrice)
	} else if verb == "nextpage" {
		return handleNext(parameters.PageToken)
	} else if verb == "photo" {
		return handlePhoto(parameters.PhotoRef)
	} else {
		return clientError(http.StatusBadRequest)
	}
}

func handleCreate(lat, long float64, radius uint, minPrice, maxPrice int) (events.APIGatewayProxyResponse, error) {
	biteArray := respondBiteArray(lat, long, radius, minPrice, maxPrice)
	return clientSuccess(biteArray), nil
}

func handleNext(pagetoken string) (events.APIGatewayProxyResponse, error) {
	biteArray := respondNextPage(pagetoken)
	jsonBiteArray, err := json.Marshal(biteArray)
	check(err)
	log.Println(string(jsonBiteArray))
	return events.APIGatewayProxyResponse{
		StatusCode:      http.StatusOK,
		Headers:         map[string]string{"Content-Type": "application/json"},
		IsBase64Encoded: false,
		Body:            string(jsonBiteArray),
	}, nil
}

func handlePhoto(photoref string) (events.APIGatewayProxyResponse, error) {
	if len(photoref) > 0 {
		photoResponse := respondPhoto(photoref)
		photo, _ := photoResponse.Image()
		var buff bytes.Buffer
		png.Encode(&buff, photo)
		encodedString := base64.StdEncoding.EncodeToString(buff.Bytes())
		log.Println(encodedString)
		return events.APIGatewayProxyResponse{
			StatusCode:      200,
			Headers:         map[string]string{"Content-Type": "application/json"},
			IsBase64Encoded: true,
			Body:            encodedString,
		}, nil
	} else {
		return clientError(http.StatusBadRequest)
	}
}

func serverError(err error) (events.APIGatewayProxyResponse, error) {
	errorLogger.Println(err.Error())

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
	}, nil
}

func clientError(status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       http.StatusText(status),
	}, nil
}

func clientSuccess(biteArray maps.PlacesSearchResponse) events.APIGatewayProxyResponse {
	jsonBiteArray, err := json.Marshal(biteArray)
	check(err)
	log.Println(string(jsonBiteArray))
	return events.APIGatewayProxyResponse{
		StatusCode:      http.StatusOK,
		Headers:         map[string]string{"Content-Type": "application/json"},
		IsBase64Encoded: false,
		Body:            string(jsonBiteArray),
	}
}

func respondBiteArray(lat float64, long float64, radius uint, minPrice int, maxPrice int) maps.PlacesSearchResponse {
	var client *maps.Client
	var err error
	client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	check(err)
	r := &maps.NearbySearchRequest{
		Radius:  radius,
		Type:    maps.PlaceTypeRestaurant,
		OpenNow: true,
	}
	parseLocation(fmt.Sprintf("%f,%f", lat, long), r)
	parsePriceLevels(minPrice, maxPrice, r)
	resp, err := client.NearbySearch(context.Background(), r)
	check(err)
	log.Println(resp)
	return resp
}

func respondNextPage(pagetoken string) maps.PlacesSearchResponse {
	var client *maps.Client
	var err error
	client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	check(err)
	r := &maps.NearbySearchRequest{
		PageToken: pagetoken,
	}
	resp, err := client.NearbySearch(context.Background(), r)
	check(err)
	return resp
}

func respondPhoto(photoref string) maps.PlacePhotoResponse {
	var client *maps.Client
	var err error
	client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	check(err)
	r := &maps.PlacePhotoRequest{
		PhotoReference: photoref,
		MaxHeight:      600,
		MaxWidth:       600,
	}
	resp, respErr := client.PlacePhoto(context.Background(), r)
	check(respErr)
	return resp
}

func parseLocation(location string, r *maps.NearbySearchRequest) {
	if location != "" {
		l, err := maps.ParseLatLng(location)
		check(err)
		r.Location = &l
	}
}

func parsePriceLevel(priceLevel int) maps.PriceLevel {
	switch priceLevel {
	case 0:
		return maps.PriceLevelFree
	case 1:
		return maps.PriceLevelInexpensive
	case 2:
		return maps.PriceLevelModerate
	case 3:
		return maps.PriceLevelExpensive
	case 4:
		return maps.PriceLevelVeryExpensive
	default:
		return maps.PriceLevelFree
	}
}

func parsePriceLevels(minPrice int, maxPrice int, r *maps.NearbySearchRequest) {
	if minPrice > 0 {
		r.MinPrice = parsePriceLevel(minPrice)
	}
	if maxPrice < 5 {
		r.MaxPrice = parsePriceLevel(minPrice)
	}
}