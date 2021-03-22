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
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"googlemaps.github.io/maps"
)

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

func router(req events.APIGatewayProxyRequest) {
	switch req.HTTPMethod {
	case "POST":
		handleRequest(req)
	default:
		clientError(http.StatusMethodNotAllowed)
	}
}

func handleRequest(req events.APIGatewayProxyRequest) {
	verb := req.QueryStringParameters["verb"]
	if verb == "create" {
		handleCreate(req)
	} else if verb == "nextpage" {
		handleNext(req)
	} else if verb == "photo" {
		handlePhoto(req)
	} else {
		clientError(http.StatusBadRequest)
	}
}

func handleCreate(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lat := req.QueryStringParameters["lat"]
	long := req.QueryStringParameters["long"]
	radius, radErr := strconv.ParseUint(req.QueryStringParameters["radius"], 10, 0)
	check(radErr)
	minPrice, minPriceErr := strconv.ParseInt(req.QueryStringParameters["minPrice"], 10, 0)
	check(minPriceErr)
	maxPrice, maxPriceErr := strconv.ParseInt(req.QueryStringParameters["maxPrice"], 10, 0)
	check(maxPriceErr)
	if len(lat) > 0 && len(long) > 0 {
		float_lat, err := strconv.ParseFloat(lat, 64)
		check(err)
		float_long, err := strconv.ParseFloat(long, 64)
		check(err)
		biteArray := respondBiteArray(float_lat, float_long, uint(radius), int(minPrice), int(maxPrice))
		return clientSuccess(biteArray)
	} else {
		return clientError(http.StatusBadRequest)
	}
}

func handleNext(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	pagetoken := req.QueryStringParameters["pagetoken"]
	if len(pagetoken) > 0 {
		biteArray := respondNextPage(pagetoken)
		return clientSuccess(biteArray)
	} else {
		return clientError(http.StatusBadRequest)
	}
}

func handlePhoto(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	photoref := req.QueryStringParameters["photoref"]
	if len(photoref) > 0 {
		photoResponse := respondPhoto(photoref)
		photo, _ := photoResponse.Image()
		var buff bytes.Buffer
		png.Encode(&buff, photo)
		encodedString := base64.StdEncoding.EncodeToString(buff.Bytes())
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       encodedString,
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

func clientSuccess(biteArray maps.PlacesSearchResponse) (events.APIGatewayProxyResponse, error) {
	jsonBiteArray, err := json.Marshal(biteArray)
	check(err)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(jsonBiteArray),
	}, nil
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
		RankBy:  maps.RankByDistance,
	}
	parseLocation(fmt.Sprintf("%f,%f", lat, long), r)
	parsePriceLevels(minPrice, maxPrice, r)
	resp, err := client.NearbySearch(context.Background(), r)
	check(err)
	errorLogger.Println(resp)
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
	errorLogger.Println(resp)
	return resp
}

func respondPhoto(photoref string) maps.PlacePhotoResponse {
	var client *maps.Client
	var err error
	client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	check(err)
	r := &maps.PlacePhotoRequest{
		PhotoReference: photoref,
	}
	resp, respErr := client.PlacePhoto(context.Background(), r)
	check(respErr)
	errorLogger.Println(resp)
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
