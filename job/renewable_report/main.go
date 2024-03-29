package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/repo"
	"goclassifieds/lib/sign"
	"goclassifieds/lib/utils"
	"goclassifieds/lib/watttime"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/opensearch-project/opensearch-go"
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

type Intensity struct {
	Region string  `json:"region"`
	Rating float64 `json:"rating"`
}

type CalculateIntensitiesInput struct {
	StartDate time.Time
	EndDate   time.Time
}

type EnergyGridCarbonIntensity struct {
	Id          string    `json:"id"`
	Region      string    `json:"region"`
	Rating      float64   `json:"rating"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	CreatedDate time.Time `json:"created_date"`
}

type ReportEntityManagerInput struct {
	Session    *session.Session
	BucketName string
}

type EnergyGridCarbonIntensityEntityManagerInput struct {
	OsClient *opensearch.Client
}

func handler(ctx context.Context, b json.RawMessage) {

	utils.LogUsageForLambda()

	frequency := 5 * time.Minute
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

	gridIntensities := make([]*EnergyGridCarbonIntensity, len(locations))

	for index, location := range locations {
		report.Intensities[regions.Regions[index].RegionName] = location.Rating
		gridIntensities[index] = &EnergyGridCarbonIntensity{
			Id:          utils.GenerateId(),
			CreatedDate: report.CreatedDate,
			StartDate:   startDate,
			EndDate:     endDate,
			Region:      regions.Regions[index].RegionName,
			Rating:      location.Rating,
		}
	}

	sess := session.Must(session.NewSession())

	userPasswordAwsSigner := sign.UserPasswordAwsSigner{
		Service:            "es",
		Region:             "us-east-1",
		Session:            sess,
		IdentityPoolId:     os.Getenv("IDENTITY_POOL_ID"),
		Issuer:             os.Getenv("ISSUER"),
		Username:           os.Getenv("DEFAULT_SIGNING_USERNAME"),
		Password:           os.Getenv("DEFAULT_SIGNING_PASSWORD"),
		CognitoAppClientId: os.Getenv("COGNITO_APP_CLIENT_ID"),
	}

	opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
		Signer:    userPasswordAwsSigner,
	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Error: %s", err.Error())
	}

	manageInput := &ReportEntityManagerInput{
		Session:    sess,
		BucketName: os.Getenv("BUCKET_NAME"),
	}
	manager := ReportEntityManager(manageInput)
	entity, err := ReportToEntity(&report)

	gridManageInput := &EnergyGridCarbonIntensityEntityManagerInput{
		OsClient: osClient,
	}
	gridManager := EnergyGridCarbonIntensityEntityManager(gridManageInput)

	if err != nil {
		log.Print("Failure converting report to entity", err.Error())
	} else {
		manager.Save(entity, "default") // s3
		manager.Save(entity, "github")
		for _, gridIntensity := range gridIntensities {
			intensitEntity, _ := EnergyGridCarbonIntensityToEntity(gridIntensity)
			gridManager.Save(intensitEntity, "default")
		}
	}
}

func ReportEntityManager(input *ReportEntityManagerInput) *entity.EntityManager {

	pem, err := os.ReadFile("api/entity/rtc-vertigo-" + os.Getenv("STAGE") + ".private-key.pem")
	if err != nil {
		log.Print("Error reading github app pem file", err.Error())
	}

	getInstallionTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        "rollthecloudinc",
		GithubAppId:  os.Getenv("GITHUB_APP_ID"),
	}
	installationToken, err := repo.GetInstallationToken(getInstallionTokenInput)
	if err != nil {
		log.Print("Error fetching installation token.")
	}

	githubToken := *installationToken.Token
	srcToken := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	httpClient := oauth2.NewClient(context.Background(), srcToken)
	githubV4Client := githubv4.NewClient(httpClient)
	manager := entity.NewDefaultManager(entity.DefaultManagerConfig{
		SingularName:   "renewable_report",
		PluralName:     "renewable_reports",
		GithubV4Client: githubV4Client,
		Stage:          os.Getenv("STAGE"),
		Session:        input.Session,
		BucketName:     input.BucketName,
	})
	suffix := ""
	if os.Getenv("STAGE") == "prod" {
		suffix = "-prod"
	}
	manager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
	manager.AddLoader("github", entity.GithubFileLoaderAdaptor{
		Config: entity.GithubFileUploadConfig{
			Client:   githubV4Client,
			Repo:     "rollthecloudinc/hedge-objects" + suffix,
			Branch:   os.Getenv("GITHUB_BRANCH"),
			Path:     "renewable-report",
			UserName: os.Getenv("DEFAULT_SIGNING_USERNAME"),
		},
	})
	manager.AddStorage("github", entity.GithubFileUploadAdaptor{
		Config: entity.GithubFileUploadConfig{
			Client:   githubV4Client,
			Repo:     "rollthecloudinc/hedge-objects" + suffix,
			Branch:   os.Getenv("GITHUB_BRANCH"),
			Path:     "renewable-report",
			UserName: os.Getenv("DEFAULT_SIGNING_USERNAME"),
		},
	})
	log.Print("create report manager")
	return &manager
}

func EnergyGridCarbonIntensityEntityManager(input *EnergyGridCarbonIntensityEntityManagerInput) *entity.EntityManager {
	manager := entity.NewDefaultManager(entity.DefaultManagerConfig{
		SingularName: "energy_grid_carbon_intensity",
		PluralName:   "energy_grid_carbon_intensities",
		Stage:        os.Getenv("STAGE"),
	})
	manager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
	manager.AddStorage("default", entity.OpensearchStorageAdaptor{
		Config: entity.OpensearchAdaptorConfig{
			Index:  "energy-grid-carbon-intensity-001",
			Client: input.OsClient,
		},
	})
	log.Print("create energy grid carbon intensity manager")
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
			wattIntensity, err := WattTimeFallbackIntensityCalculation(region.RegionName)
			if err == nil {
				locations[index] = Location{Location: "", Rating: wattIntensity}
			}
			continue
		}
		body, _ := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Print("Error reading calculation response for "+region.RegionName, err.Error())
			wattIntensity, err := WattTimeFallbackIntensityCalculation(region.RegionName)
			if err == nil {
				locations[index] = Location{Location: "", Rating: wattIntensity}
			}
			continue
		}
		log.Print(string(body))
		var resLocations []Location
		err = json.Unmarshal(body, &resLocations)
		if err != nil {
			log.Print("Error marshalling calculation response for "+region.RegionName, err.Error())
			wattIntensity, err := WattTimeFallbackIntensityCalculation(region.RegionName)
			if err == nil {
				locations[index] = Location{Location: "", Rating: wattIntensity}
			}
			continue
		}
		if len(resLocations) == 0 {
			log.Print("Response empty for "+region.RegionName, err.Error())
			wattIntensity, err := WattTimeFallbackIntensityCalculation(region.RegionName)
			if err == nil {
				locations[index] = Location{Location: "", Rating: wattIntensity}
			}
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

func EnergyGridCarbonIntensityToEntity(gridIntensity *EnergyGridCarbonIntensity) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(gridIntensity); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(gridIntensity)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func WattTimeFallbackIntensityCalculation(name string) (float64, error) {
	var ba = ""
	if name == "norway" {
		ba = "NO"
	}
	if ba == "" {
		return 0, errors.New("unimplemented watttime name")
	}
	loginInput := &watttime.LoginInput{
		Username: os.Getenv("WATTTIME_USERNAME"),
		Password: os.Getenv("WATTTIME_PASSWORD"),
	}
	loginRes, err := watttime.Login(loginInput)
	if err != nil {
		return 0, err
	}
	indexInput := &watttime.IndexInput{
		Token: loginRes.Token,
		Ba:    ba,
	}
	indexRes, err := watttime.GetIndex(indexInput)
	if err != nil {
		return 0, err
	} else {
		return indexRes.Moer, nil
	}
}

func main() {
	log.SetFlags(0)
	log.Print("renewable_report start")
	lambda.Start(handler)
}
