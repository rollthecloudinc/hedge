package utils

import "github.com/google/uuid"

func GenerateId() string {
	uuid, _ := uuid.NewUUID()
	return uuid.String()
}
