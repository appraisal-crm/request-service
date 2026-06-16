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
	ID          uuid.UUID
	ClientID    uuid.UUID
	InspectorID *uuid.UUID
	ObjectType  *ObjectType
	Address     *string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
