package tixer

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type (

	// TicketID represents a unique identifier for a ticket.
	TicketID uuid.UUID

	// Ticket represents an individual ticket in the system.
	Ticket struct {
		ID          TicketID
		Title       string
		Price       float64
		DateCreated time.Time
		DateUpdated time.Time
	}

	Filter struct {
		Before TicketID
		After  TicketID
		Limit  int
	}

	Metadata struct {
		Before TicketID
		After  TicketID
		Total  int
	}

	// TicketService represents a service for managing tickets.
	TicketService interface {
		CreateTicket(ctx context.Context, ticket Ticket) error
		ReadTicket(ctx context.Context, id TicketID) (Ticket, error)
		UpdateTicket(ctx context.Context, ticket Ticket) (Ticket, error)
		DeleteTicket(ctx context.Context, id TicketID) error
		ReadTickets(ctx context.Context, filter Filter) ([]Ticket, Metadata, error)
	}

	Validator interface {
		Valid() bool
		Check(ok bool, key, message string)
		AddError(key, message string)
	}
)

func (t Ticket) Validate(vld Validator) {
	t.ValidateTitle(vld)
	t.ValidatPrice(vld)
}

func (t Ticket) ValidateTitle(vld Validator) {
	vld.Check(t.Title != "", "title", "must be provided")
	vld.Check(len(t.Title) <= 50, "title", "must not be longer than 50 characters")
}

func (t Ticket) ValidatPrice(vld Validator) {
	vld.Check(t.Price > 0 && t.Price <= 100_000, "price", "must be in the range [0, 100 000]")
}

func NewTicketID() TicketID {
	return TicketID(uuid.New())
}

func (id TicketID) String() string {
	return uuid.UUID(id).String()
}
