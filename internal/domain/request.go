package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusNew                 Status = "new"
	StatusInProgress          Status = "in_progress"
	StatusInspectionScheduled Status = "inspection_scheduled"
	StatusInspectionCompleted Status = "inspection_completed"
	StatusAppraisal           Status = "appraisal"
	StatusReportSent          Status = "report_sent"
	StatusClosed              Status = "closed"
)

type ObjectType string

const (
	ObjectTypeApartment  ObjectType = "apartment"
	ObjectTypeHouse      ObjectType = "house"
	ObjectTypeLand       ObjectType = "land"
	ObjectTypeCommercial ObjectType = "commercial"
	ObjectTypeCar        ObjectType = "car"
)

type Request struct {
	ID          uuid.UUID   `json:"id"`
	ClientID    uuid.UUID   `json:"client_id"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phone_number"`
	InspectorID *uuid.UUID  `json:"inspector_id,omitempty"`
	ObjectType  *ObjectType `json:"object_type,omitempty"`
	Address     *string     `json:"address,omitempty"`
	Status      Status      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
