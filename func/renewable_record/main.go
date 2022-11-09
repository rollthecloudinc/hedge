package main

import (
	"bytes"
	"context"
	"encoding/json"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/sign"
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
	RequestId      string `json:"request_id"`
	Region         string `json:"region"`
	Duration       uint16 `json:"duration"`
	BilledDuration uint16 `json:"billed_duration"`
	MemorySize     uint16 `json:"memory_size"`
	MaxMemoryUsed  uint16 `json:"max_memory_used"`
	InitDuration   uint16 `json:"init_duration"`
	Intensity      uint16 `json:"intensity"`
	Electricity    uint16 `json:"electricity"`
	Function       string `json:"function"`
	Path           string `json:"path"`
}

type RenewableRecordEntityManagerInput struct {
	OsClient *opensearch.Client
}

func handler(ctx context.Context, logsEvent events.CloudwatchLogsEvent) {
	data, _ := logsEvent.AWSLogs.Parse()
	records := make([]RenewableRecord, 0)
	for _, logEvent := range data.LogEvents {
		log.Print(logEvent.Message)
		pieces := strings.Fields(logEvent.Message)
		lastNumberIndex := -1
		record := RenewableRecord{
			Region: os.Getenv("AWS_REGION"),
		}
		for index, field := range pieces {
			_, err := strconv.ParseFloat(field, 32)
			ival, err2 := strconv.ParseUint(field, 0, 16)
			if err == nil && err2 == nil {
				if lastNumberIndex == -1 && len(pieces[index-2]) > 30 {
					lastNumberIndex = index - 3
				}
				name := strings.Join(pieces[lastNumberIndex+2:index], " ")
				if name == "Duration:" {
					// record.Duration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
					record.Duration = uint16(ival)
				} else if name == "Billed Duration:" {
					// record.BilledDuration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
					record.BilledDuration = uint16(ival)
				} else if name == "Memory Size:" {
					// record.MemorySize = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
					record.MemorySize = uint16(ival)
				} else if name == "Max Memory Used:" {
					// record.MaxMemoryUsed = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
					record.MaxMemoryUsed = uint16(ival)
				} else if name == "Init Duration:" {
					// record.InitDuration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
					record.InitDuration = uint16(ival)
				}
				lastNumberIndex = index
			} else if field == "RequestId:" && len(pieces) >= index {
				record.RequestId = pieces[index+1]
			} else if field == "Duration:" && len(pieces) >= index {
				if pieces[index-1] == "Billed" {
					// record.BilledDuration = strings.Join(pieces[index+1:index+3], "")
					f, _ := strconv.ParseFloat(pieces[index+1], 32)
					record.BilledDuration = uint16(math.Round(f))
				} else if pieces[index-1] == "Init" {
					// record.InitDuration = strings.Join(pieces[index+1:index+3], "")
					f, _ := strconv.ParseFloat(pieces[index+1], 32)
					record.InitDuration = uint16(math.Round(f))
				} else {
					// record.Duration = strings.Join(pieces[index+1:index+3], "")
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
			}
		}
		b, err := json.Marshal(record)
		if err == nil {
			log.Print(string(b))
			records = append(records, record)
		} else {
			log.Print("json marshall failure")
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
		for _, r := range records {
			recordEntity, _ := RenewableRecordToEntity(&r)
			recordManager.Save(recordEntity, "default")
		}
	}
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
