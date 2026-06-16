package handler

import (
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/domain"
	"github.com/google/uuid"
)

type createRequestDTO struct {
	ObjectType *domain.ObjectType `json:"object_type"`
	Address    *string            `json:"address"`
}

type updateRequestDTO struct {
	InspectorID *uuid.UUID         `json:"inspector_id"`
	ObjectType  *domain.ObjectType `json:"object_type"`
	Address     *string            `json:"address"`
}

type changeStatusDTO struct {
	Status domain.Status `json:"status"`
}
