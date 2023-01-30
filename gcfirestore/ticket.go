// Package gcfirestore implements ticket service over Google Cloud Firestore.
package gcfirestore

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/mroobert/tixer-tickets"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrCounterNotFound = errors.New("counter not found")

// Storer persists tickets in Firestore.
type Storer struct {
	client       *firestore.Client
	collection   string
	counterDocID string
}

func NewStorer(client *firestore.Client, collection, counterDocID string) *Storer {
	return &Storer{
		client,
		collection,
		counterDocID,
	}
}

// CreateTicket creates a ticket in Firestore.
//
// It uses a transaction to ensure atomicity regarding
// the creation of the ticket and the increment of the totalTickets field.
func (s *Storer) CreateTicket(ctx context.Context, ticket tixer.Ticket) error {
	tRef := s.client.Collection(s.collection).Doc(ticket.ID.String())
	cRef := s.client.Collection(s.collection).Doc(s.counterDocID)

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := tx.Create(tRef, createTicket{
			Title: ticket.Title,
			Price: ticket.Price,
		})
		if err != nil {
			return err
		}

		err = tx.Update(cRef, []firestore.Update{
			{Path: "totalTickets", Value: firestore.Increment(1)},
		})

		return err
	})

	return err
}

func (s *Storer) ReadTicket(ctx context.Context, id tixer.TicketID) (tixer.Ticket, error) {
	return s.readTicket(ctx, id)
}

// UpdateTicket updates a ticket in Firestore.
//
// It uses a transaction to ensure no data races occur.
//
// It makes an extra read to retrieve the updated ticket.
func (s *Storer) UpdateTicket(ctx context.Context, ticket tixer.Ticket) (tixer.Ticket, error) {
	dRef := s.client.Collection(s.collection).Doc(ticket.ID.String())
	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := tx.Get(dRef)
		if err != nil {
			switch {
			case status.Code(err) == codes.NotFound:
				return tixer.ErrTicketNotFound
			default:
				return err
			}
		}

		updates := []firestore.Update{
			{Path: "dateUpdated", Value: firestore.ServerTimestamp},
		}
		if ticket.Title != "" {
			updates = append(updates, firestore.Update{
				Path:  "title",
				Value: ticket.Title,
			})
		}
		if ticket.Price != 0 {
			updates = append(updates, firestore.Update{
				Path:  "price",
				Value: ticket.Price,
			})
		}

		return tx.Update(dRef, updates)
	})
	if err != nil {
		return tixer.Ticket{}, err
	}

	return s.readTicket(ctx, ticket.ID)
}

func (s *Storer) DeleteTicket(ctx context.Context, id tixer.TicketID) error {
	tRef := s.client.Collection(s.collection).Doc(id.String())
	cRef := s.client.Collection(s.collection).Doc(s.counterDocID)

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := tx.Delete(tRef)
		if err != nil {
			return err
		}

		err = tx.Update(cRef, []firestore.Update{
			{Path: "totalTickets", Value: firestore.Increment(-1)},
		})

		return err
	})

	return err
}

func (s *Storer) ReadTickets(ctx context.Context, filter tixer.Filter) ([]tixer.Ticket, tixer.Metadata, error) {
	query := s.client.Collection(s.collection).OrderBy("dateCreated", firestore.Desc).Limit(filter.Limit)

	if filter.After.String() != uuid.Nil.String() {
		afterDoc, err := s.client.Collection(s.collection).Doc(filter.After.String()).Get(ctx)
		if err != nil {
			switch {
			case status.Code(err) == codes.NotFound:
				return nil, tixer.Metadata{}, tixer.ErrTicketNotFound
			default:
				return nil, tixer.Metadata{}, err
			}
		}
		query = query.StartAfter(afterDoc)
	}
	if filter.Before.String() != uuid.Nil.String() {
		beforeDoc, err := s.client.Collection(s.collection).Doc(filter.Before.String()).Get(ctx)
		if err != nil {
			switch {
			case status.Code(err) == codes.NotFound:
				return nil, tixer.Metadata{}, tixer.ErrTicketNotFound
			default:
				return nil, tixer.Metadata{}, err
			}
		}
		query = query.EndBefore(beforeDoc)
	}

	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, tixer.Metadata{}, err
	}

	var tt []tixer.Ticket
	for _, doc := range docs {
		tck, err := docToPersistedTicket(doc)
		if err != nil {
			return nil, tixer.Metadata{}, err
		}

		tt = append(tt, toDomainTicket(tck))
	}

	counterDoc, err := s.client.Collection(s.collection).Doc(s.counterDocID).Get(ctx)
	if err != nil {
		switch {
		case status.Code(err) == codes.NotFound:
			return nil, tixer.Metadata{}, ErrCounterNotFound
		default:
			return nil, tixer.Metadata{}, err
		}
	}
	cnt, err := docToPersistedCounter(counterDoc)
	if err != nil {
		return nil, tixer.Metadata{}, err
	}

	var after, before tixer.TicketID
	if len(tt) > 0 {
		before = tt[0].ID
		after = tt[len(tt)-1].ID
	}

	return tt, tixer.Metadata{
		After:  after,
		Before: before,
		Total:  cnt.TotalTickets,
	}, nil
}

func (s *Storer) readTicket(ctx context.Context, id tixer.TicketID) (tixer.Ticket, error) {
	ticketDoc, err := s.client.Collection(s.collection).Doc(id.String()).Get(ctx)
	if err != nil {
		switch {
		case status.Code(err) == codes.NotFound:
			return tixer.Ticket{}, tixer.ErrTicketNotFound
		default:
			return tixer.Ticket{}, err
		}
	}

	t, err := docToPersistedTicket(ticketDoc)
	if err != nil {
		return tixer.Ticket{}, err
	}

	return toDomainTicket(t), nil
}

type (
	// persistedTicket represents a stored ticket in Firestore.
	persistedTicket struct {
		ID          string    `firestore:"id"`
		Title       string    `firestore:"title"`
		Price       float64   `firestore:"price"`
		DateCreated time.Time `firestore:"dateCreated"`
		DateUpdated time.Time `firestore:"dateUpdated"`
	}

	// counter represents the total tickets counter.
	persistedCounter struct {
		TotalTickets int `firestore:"totalTickets"`
	}

	// createTicket contains the data needed to create a Ticket in Firestore.
	createTicket struct {
		Title       string    `firestore:"title"`
		Price       float64   `firestore:"price"`
		DateCreated time.Time `firestore:"dateCreated,serverTimestamp"`
	}
)

func toDomainTicket(t persistedTicket) tixer.Ticket {
	return tixer.Ticket{
		ID:          tixer.TicketID(uuid.MustParse(t.ID)),
		Title:       t.Title,
		Price:       t.Price,
		DateCreated: t.DateCreated,
		DateUpdated: t.DateUpdated,
	}
}

func docToPersistedTicket(doc *firestore.DocumentSnapshot) (persistedTicket, error) {
	var tck persistedTicket
	if err := doc.DataTo(&tck); err != nil {
		return tck, err
	}
	tck.ID = doc.Ref.ID

	return tck, nil
}

func docToPersistedCounter(doc *firestore.DocumentSnapshot) (persistedCounter, error) {
	var cnt persistedCounter
	if err := doc.DataTo(&cnt); err != nil {
		return cnt, err
	}

	return cnt, nil
}
