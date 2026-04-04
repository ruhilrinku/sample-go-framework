package common

import (
	"github.com/google/uuid"
)

func GenerateUUID() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}
