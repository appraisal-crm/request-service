package handler

import (
	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/google/uuid"
)

type createRequestDTO struct {
	Email       string             `json:"email"        validate:"required,email"`
	PhoneNumber string             `json:"phone_number" validate:"required,e164"`
	ObjectType  *domain.ObjectType `json:"object_type"  validate:"omitempty,oneof=apartment house land commercial car"`
	Address     *string            `json:"address"      validate:"omitempty,min=1"`
}

type updateRequestDTO struct {
	InspectorID *uuid.UUID         `json:"inspector_id"`
	ObjectType  *domain.ObjectType `json:"object_type" validate:"omitempty,oneof=apartment house land commercial car"`
	Address     *string            `json:"address"     validate:"omitempty,min=1"`
}

type changeStatusDTO struct {
	Status domain.Status `json:"status" validate:"required,oneof=new in_progress inspection_scheduled inspection_completed appraisal report_sent closed"`
}

type listAllResponse struct {
	Data  []*domain.Request `json:"data"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}
