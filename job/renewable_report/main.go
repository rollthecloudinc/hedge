package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/entity"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

var regions Regions
var locations []Location

type Regions struct {
	Id      string   `json:"id"`
	Regions []Region `json:"regions"`
}

type Region struct {
	RegionName string `json:"RegionName"`
}

type Location struct {
	Location string  `json:"location"`
	Rating   float64 `json:"rating"`
}

type Report struct {
	Id          string             `json:"id"`
	Intensities map[string]float64 `json:"intensities"`
	UserId      string             `json:"userId"`
	StartDate   time.Time          `json:"startDate"`
	EndDate     time.Time          `json:"endDate"`
	CreatedDate time.Time          `json:"createdDate"`
}

type CalculateIntensitiesInput struct {
	StartDate time.Time
	EndDate   time.Time
}

func handler(ctx context.Context, b json.RawMessage) {
	log.Print("renewable_report run")

	frequency := 4 * time.Hour
	startDate := time.Now()
	endDate := time.Now().Add(frequency)

	ReadRegions()

	calcIntensitiesInput := &CalculateIntensitiesInput{
		StartDate: startDate,
		EndDate:   endDate,
	}
	CalculateIntensities(calcIntensitiesInput)

	report := Report{}
	report.Id = "report"
	report.CreatedDate = time.Now()
	report.StartDate = startDate
	report.EndDate = endDate
	report.Intensities = make(map[string]float64)

	for index, location := range locations {
		report.Intensities[regions.Regions[index].RegionName] = location.Rating
	}

	manager := ReportEntityManager()
	entity, err := ReportToEntity(&report)

	if err != nil {
		log.Print("Failure converting report to entity", err.Error())
	} else {
		manager.Save(entity, "default")
	}
}

func ReportEntityManager() *entity.EntityManager {
	var githubToken string
	var srcToken oauth2.TokenSource
	githubToken = os.Getenv("GITHUB_TOKEN")
	srcToken = oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	httpClient := oauth2.NewClient(context.Background(), srcToken)
	githubV4Client := githubv4.NewClient(httpClient)
	manager := entity.NewDefaultManager(entity.DefaultManagerConfig{
		SingularName:   "renewable_report",
		PluralName:     "renewable_reports",
		GithubV4Client: githubV4Client,
		Stage:          os.Getenv("STAGE"),
	})
	manager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
	manager.AddLoader("default", entity.GithubFileLoaderAdaptor{
		Config: entity.GithubFileUploadConfig{
			Client:   githubV4Client,
			Repo:     "rollthecloudinc/hedge-objects",
			Branch:   os.Getenv("GITHUB_BRANCH"),
			Path:     "renewable-report",
			UserName: "ng-druid",
		},
	})
	manager.AddStorage("default", entity.GithubFileUploadAdaptor{
		Config: entity.GithubFileUploadConfig{
			Client:   githubV4Client,
			Repo:     "rollthecloudinc/hedge-objects",
			Branch:   os.Getenv("GITHUB_BRANCH"),
			Path:     "renewable-report",
			UserName: "ng-druid",
		},
	})
	log.Print("create report manager")
	return &manager
}

func ReadRegions() {
	rawRegions, err := os.ReadFile("job/renewable_report/regions.json")
	if err != nil {
		log.Print("Error reading regions file", err.Error())
	}
	err = json.Unmarshal(rawRegions, &regions)
	if err != nil {
		log.Print("Error unmarshalling regions")
	}
}

func CalculateIntensities(input *CalculateIntensitiesInput) {
	locations = make([]Location, len(regions.Regions))
	format := "%d-%02d-%02dT%02d:%02d:%02d"
	startDate := fmt.Sprintf(format,
		input.StartDate.Year(), input.StartDate.Month(), input.StartDate.Day(),
		input.StartDate.Hour(), input.StartDate.Minute(), input.StartDate.Second())
	endDate := fmt.Sprintf(format,
		input.EndDate.Year(), input.EndDate.Month(), input.EndDate.Day(),
		input.EndDate.Hour(), input.EndDate.Minute(), input.EndDate.Second())
	baseUri := "https://carbon-aware-api.azurewebsites.net/emissions/bylocation?time=" + url.QueryEscape(startDate) + "&toTime=" + url.QueryEscape(endDate)
	for index, region := range regions.Regions {
		uri := baseUri + "&location=" + region.RegionName
		log.Print("GET: " + uri)
		res, err := http.Get(uri)
		if err != nil {
			log.Print("Calculation of intensities failed for "+region.RegionName, err.Error())
			continue
		}
		body, _ := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Print("Error reading calculation response for "+region.RegionName, err.Error())
			continue
		}
		log.Print(string(body))
		var resLocations []Location
		err = json.Unmarshal(body, &resLocations)
		if err != nil {
			log.Print("Error marshalling calculation response for "+region.RegionName, err.Error())
			continue
		}
		if len(resLocations) == 0 {
			log.Print("Response empty for "+region.RegionName, err.Error())
			continue
		}
		locations[index] = resLocations[0]
	}
}

func ReportToEntity(report *Report) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(report); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func main() {
	log.Print("renewable_report start")
	lambda.Start(handler)
}
