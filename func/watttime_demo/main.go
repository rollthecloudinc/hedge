package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"goclassifieds/lib/watttime"

	"github.com/aws/aws-lambda-go/lambda"
)

type RegionIntensity struct {
	Name      string
	Ba        string
	Intensity int
}

func handler(ctx context.Context, b json.RawMessage) {
	log.Print("top handler")

	loginInput := &watttime.LoginInput{
		Username: os.Getenv("WATTTIME_USERNAME"),
		Password: os.Getenv("WATTTIME_PASSWORD"),
	}

	loginRes, err := watttime.Login(loginInput)
	if err != nil {
		log.Print(err.Error())
	}

	// log.Printf("token = %s", loginRes.Token)

	regionsInput := &watttime.GridRegionsInput{
		Token: loginRes.Token,
	}

	gridRegionsRes, err := watttime.GridRegions(regionsInput)
	if err != nil {
		log.Print(err.Error())
	}

	intensities := make([]*RegionIntensity, len(gridRegionsRes.GridRegions))

	for index, region := range gridRegionsRes.GridRegions {
		log.Print("region: " + region.Name + " | Ba: " + region.Ba)
		indexInput := &watttime.IndexInput{
			Token: loginRes.Token,
			Ba:    region.Ba,
		}
		intensity := &RegionIntensity{
			Name: region.Name,
			Ba:   region.Ba,
		}
		intensities[index] = intensity
		indexRes, err := watttime.GetIndex(indexInput)
		if err != nil {
			log.Print(err.Error())
		} else {
			intensity.Intensity = indexRes.Percent
			log.Printf("Grid Intensity: %d", indexRes.Percent)
		}
	}
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
