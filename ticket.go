package tixer

import (
	"context"
	"time"
)

// Ticket represents an individual ticket.
// It's a domain type.
type Ticket struct {
	ID        string
	Title     string
	Price     float64
	CreatedAt time.Time
}

// TicketService represents a service for managing tickets.
type TicketService interface {
	// CreateTicket creates a new ticket.
	CreateTicket(ctx context.Context, ticket *Ticket) error

	// ReadTicket retrieves a ticket by its ID.
	ReadTicket(ctx context.Context, id string) (*Ticket, error)

	// UpdateTicket updates a ticket.
	UpdateTicket(ctx context.Context, ticket *Ticket) error

	// DeleteTicket deletes a ticket.
	DeleteTicket(ctx context.Context, id string) error
}
