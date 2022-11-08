package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type RenewableRecord struct {
	RequestId      string `json:"request_id"`
	Region         string `json:"region"`
	Duration       string `json:"duration"`
	BilledDuration string `json:"billed_duration"`
	MemorySize     string `json:"memory_size"`
	MaxMemoryUsed  string `json:"max_memory_used"`
	InitDuration   string `json:"init_duration"`
	Intensity      string `json:"intensity"`
	Electricity    string `json:"electricity"`
	Function       string `json:"function"`
	Path           string `json:"path"`
}

func handler(ctx context.Context, logsEvent events.CloudwatchLogsEvent) {
	data, _ := logsEvent.AWSLogs.Parse()
	for _, logEvent := range data.LogEvents {
		pieces := strings.Fields(logEvent.Message)
		lastNumberIndex := -1
		record := RenewableRecord{
			Region: os.Getenv("AWS_REGION"),
		}
		for index, field := range pieces {
			val, err := strconv.ParseFloat(field, 32)
			_, err2 := strconv.Atoi(field)
			if err == nil && err2 == nil {
				if lastNumberIndex == -1 && len(pieces[index-2]) > 30 {
					lastNumberIndex = index - 3
				}
				name := strings.Join(pieces[lastNumberIndex+2:index], " ")
				if name == "Duration:" {
					record.Duration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
				} else if name == "Billed Duration:" {
					record.BilledDuration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
				} else if name == "Memory Size:" {
					record.MemorySize = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
				} else if name == "Max Memory Used:" {
					record.MaxMemoryUsed = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
				} else if name == "Init Duration:" {
					record.InitDuration = fmt.Sprintf("%f%s", val, strings.ToLower(pieces[index+1]))
				}
				lastNumberIndex = index
			} else if field == "RequestId:" && len(pieces) >= index {
				record.RequestId = pieces[index+1]
			} else if field == "Duration:" && len(pieces) >= index {
				if pieces[index-1] == "Billed" {
					record.BilledDuration = strings.Join(pieces[index+1:index+3], "")
				} else if pieces[index-1] == "Init" {
					record.InitDuration = strings.Join(pieces[index+1:index+3], "")
				} else {
					record.Duration = strings.Join(pieces[index+1:index+3], "")
				}
			} else if field == "Function:" && len(pieces) >= index {
				record.Function = pieces[index+1]
			} else if field == "Path:" && len(pieces) >= index {
				record.Path = pieces[index+1]
			} else if field == "Path:" && len(pieces) >= index {
				record.Intensity = pieces[index+1]
			}
		}
		b, err := json.Marshal(record)
		if err == nil {
			log.Print(string(b))
		} else {
			log.Print("json marshall failure")
		}
	}
}

func main() {
	log.Print("start")
	lambda.Start(handler)
}
