package main

import (
	"bytes"
	"context"
	"encoding/json"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/sign"
	"goclassifieds/lib/utils"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/opensearch-project/opensearch-go"
)

type RenewableRecord struct {
	Id             string  `json:"id"`
	RequestId      string  `json:"request_id"`
	AwsRegion      string  `json:"aws_region"`
	Region         string  `json:"region"`
	Duration       uint16  `json:"duration"`
	BilledDuration uint16  `json:"billed_duration"`
	MemorySize     uint16  `json:"memory_size"`
	MaxMemoryUsed  uint16  `json:"max_memory_used"`
	InitDuration   uint16  `json:"init_duration"`
	Intensity      uint16  `json:"intensity"`
	Electricity    float64 `json:"electricity"`
	Carbon         float64 `json:"carbon"`
	Function       string  `json:"function"`
	Path           string  `json:"path"`
	Called         bool    `json:"called"`
	Organization   string  `json:"organization"`
	Repository     string  `json:"repository"`
	Service        string  `json:"service"`
	Resource       string  `json:"resource"`
}

type RenewableRecordEntityManagerInput struct {
	OsClient *opensearch.Client
}

type CalulcateCarbonInput struct {
	Intensity  uint16 `json:"intensity"`
	MemorySize uint16 `json:"memory_size"`
	Duration   uint16 `json:"duration"`
}

type CalulcateCarbonOutput struct {
	Carbon      float64 `json:"carbon"`
	Electricity float64 `json:"electricity"`
}

func handler(ctx context.Context, logsEvent events.CloudwatchLogsEvent) {
	log.Print("REPORT Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))

	data, _ := logsEvent.AWSLogs.Parse()
	record := RenewableRecord{
		Id:        utils.GenerateId(),
		AwsRegion: os.Getenv("AWS_REGION"),
		Called:    true,
	}
	regions := make([]string, 0)
	intensities := make([]uint16, 0)
	for _, logEvent := range data.LogEvents {
		log.Print(logEvent.Message)
		pieces := strings.Fields(logEvent.Message)
		lastNumberIndex := -1
		for index, field := range pieces {
			_, err := strconv.ParseFloat(field, 32)
			ival, err2 := strconv.ParseUint(field, 0, 16)
			if err == nil && err2 == nil {
				if lastNumberIndex == -1 && len(pieces[index-2]) > 30 {
					lastNumberIndex = index - 3
				}
				name := strings.Join(pieces[lastNumberIndex+2:index], " ")
				if name == "Duration:" {
					record.Duration = uint16(ival)
				} else if name == "Billed Duration:" {
					record.BilledDuration = uint16(ival)
				} else if name == "Memory Size:" {
					record.MemorySize = uint16(ival)
				} else if name == "Max Memory Used:" {
					record.MaxMemoryUsed = uint16(ival)
				} else if name == "Init Duration:" {
					record.InitDuration = uint16(ival)
				}
				lastNumberIndex = index
			} else if field == "RequestId:" && len(pieces) >= index {
				record.RequestId = pieces[index+1]
			} else if field == "Duration:" && len(pieces) >= index {
				if pieces[index-1] == "Billed" {
					f, _ := strconv.ParseFloat(pieces[index+1], 32)
					record.BilledDuration = uint16(math.Round(f))
				} else if pieces[index-1] == "Init" {
					f, _ := strconv.ParseFloat(pieces[index+1], 32)
					record.InitDuration = uint16(math.Round(f))
				} else {
					f, _ := strconv.ParseFloat(pieces[index+1], 32)
					record.Duration = uint16(math.Round(f))
				}
			} else if field == "Function:" && len(pieces) >= index {
				record.Function = pieces[index+1]
			} else if field == "Path:" && len(pieces) >= index {
				record.Path = pieces[index+1]
			} else if field == "Path:" && len(pieces) >= index {
				f, _ := strconv.ParseFloat(pieces[index+1], 32)
				record.Intensity = uint16(math.Round(f))
			} else if field == "X-HEDGE-REGIONS:" && len(pieces) >= index {
				for _, region := range strings.Split(pieces[index+1], ",") {
					regions = append(regions, region)
				}
			} else if field == "X-HEDGE-INTENSITIES:" && len(pieces) >= index {
				for _, intensity := range strings.Split(pieces[index+1], ",") {
					f, _ := strconv.ParseFloat(intensity, 32)
					intensities = append(intensities, uint16(math.Round(f)))
				}
			} else if field == "X-HEDGE-REGION:" && len(pieces) >= index {
				record.Region = pieces[index+1]
			} else if field == "Organization:" && len(pieces) >= index {
				record.Organization = pieces[index+1]
			} else if field == "Repository:" && len(pieces) >= index {
				record.Repository = pieces[index+1]
			} else if field == "X-HEDGE-SERVICE:" && len(pieces) >= index {
				record.Service = pieces[index+1]
			} else if field == "Resource:" && len(pieces) >= index {
				record.Resource = pieces[index+1]
			}
		}
	}

	b, err := json.Marshal(record)
	if err == nil {
		log.Print(string(b))
	} else {
		log.Print("json marshall failure")
	}

	if len(regions) == 1 {

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
	recordManageInput := &RenewableRecordEntityManagerInput{
		OsClient: osClient,
	}

	recordManager := RenewableRecordEntityManager(recordManageInput)

	for index, item := range regions {
		if item == record.Region {
			record.Intensity = intensities[index]
			ccInput := &CalulcateCarbonInput{
				Intensity:  record.Intensity,
				MemorySize: record.MemorySize,
				Duration:   record.Duration,
			}
			cc := CalulcateCarbon(ccInput)
			record.Carbon = cc.Carbon
			record.Electricity = cc.Electricity
			break
		}
	}

	recordEntity, _ := RenewableRecordToEntity(&record)
	recordManager.Save(recordEntity, "default")
	for index, item := range regions {
		if item != record.Region {
			record2 := RenewableRecord{
				Id:        utils.GenerateId(),
				RequestId: record.RequestId,
				//AwsRegion:      record.AwsRegion,
				Region:         item,
				Duration:       record.Duration,
				BilledDuration: record.BilledDuration,
				MemorySize:     record.MemorySize,
				MaxMemoryUsed:  record.MaxMemoryUsed,
				InitDuration:   record.InitDuration,
				Intensity:      intensities[index],
				Function:       record.Function,
				Path:           record.Path,
				Called:         false,
				Organization:   record.Organization,
				Repository:     record.Repository,
				Resource:       record.Resource,
				Service:        record.Service,
			}
			ccInput := &CalulcateCarbonInput{
				Intensity:  record2.Intensity,
				MemorySize: record.MemorySize,
				Duration:   record.Duration,
			}
			cc := CalulcateCarbon(ccInput)
			record2.Carbon = cc.Carbon
			record2.Electricity = cc.Electricity
			recordEntity2, _ := RenewableRecordToEntity(&record2)
			recordManager.Save(recordEntity2, "default")
		}
	}

}

func CalulcateCarbon(input *CalulcateCarbonInput) *CalulcateCarbonOutput {
	output := &CalulcateCarbonOutput{}
	minWattsAverage := .74
	maxWattsAverage := 3.5
	averageCpuUtilization := float64(50)
	averageWatts := float64(minWattsAverage + (averageCpuUtilization/100)*(maxWattsAverage-minWattsAverage))
	durationInS := float64(input.Duration / 1000 % 60)
	memorySetInMB := float64(input.MemorySize)
	functionWatts := float64(averageWatts * durationInS / 3600 * memorySetInMB / 1792)
	output.Electricity = functionWatts
	output.Carbon = functionWatts * float64(input.Intensity)
	return output
}

func RenewableRecordEntityManager(input *RenewableRecordEntityManagerInput) *entity.EntityManager {
	manager := entity.NewDefaultManager(entity.DefaultManagerConfig{
		SingularName: "renewable_record",
		PluralName:   "renewable_records",
		Stage:        os.Getenv("STAGE"),
	})
	manager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
	manager.AddStorage("default", entity.OpensearchStorageAdaptor{
		Config: entity.OpensearchAdaptorConfig{
			Index:  "renewable-record-001",
			Client: input.OsClient,
		},
	})
	log.Print("create renewable record manager")
	return &manager
}

func RenewableRecordToEntity(record *RenewableRecord) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(record); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func main() {
	log.Print("start")
	lambda.Start(handler)
}
