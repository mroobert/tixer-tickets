package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/mroobert/tixer-pkgs/validate"
	"github.com/mroobert/tixer-pkgs/web"
	"github.com/mroobert/tixer-tickets"
)

func (s *Server) registerTicketsRoutesV1(router *httprouter.Router) {
	router.HandlerFunc(http.MethodGet, "/v1/tickets", s.handleReadTickets)

	router.HandlerFunc(http.MethodGet, "/v1/tickets/:id", s.handleReadTicket)

	router.HandlerFunc(http.MethodPost, "/v1/tickets", s.handleCreateTicket)

	router.HandlerFunc(http.MethodPatch, "/v1/tickets/:id", s.handleUpdateTicket)

	router.HandlerFunc(http.MethodDelete, "/v1/tickets/:id", s.handleDeleteTicket)
}

func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	var input createTicket
	err := web.ReadJSON(w, r, &input)
	if err != nil {
		web.BadRequestResponse(s.Logger, w, r, err)
		return
	}

	tck := tixer.Ticket{
		ID:    tixer.NewTicketID(),
		Title: input.Title,
		Price: input.Price,
	}

	vld := validate.NewValidator()
	if tck.Validate(vld); !vld.Valid() {
		web.FailedValidationResponse(s.Logger, w, r, vld.Errors)
		return
	}

	err = s.TicketService.CreateTicket(r.Context(), tck)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/tickets/%s", tck.ID))

	err = web.WriteJSON(w, http.StatusCreated, web.Envelope{"ticket": mapTicketToResponse(tck)}, headers)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
	}
}

func (s *Server) handleReadTicket(w http.ResponseWriter, r *http.Request) {
	id, err := web.ReadIDParam(r)
	if err != nil {
		web.BadRequestResponse(s.Logger, w, r, err)
		return
	}

	tck, err := s.TicketService.ReadTicket(r.Context(), tixer.TicketID(id))
	if err != nil {
		switch {
		case errors.Is(err, tixer.ErrTicketNotFound):
			web.NotFoundResponse(s.Logger, w, r)
		default:
			web.ServerErrorResponse(s.Logger, w, r, err)
		}

		return
	}

	err = web.WriteJSON(w, http.StatusOK, web.Envelope{"ticket": mapTicketToResponse(tck)}, nil)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
		return
	}
}

func (s *Server) handleUpdateTicket(w http.ResponseWriter, r *http.Request) {
	id, err := web.ReadIDParam(r)
	if err != nil {
		web.BadRequestResponse(s.Logger, w, r, err)
		return
	}

	var input updateTicket
	err = web.ReadJSON(w, r, &input)
	if err != nil {
		web.BadRequestResponse(s.Logger, w, r, err)
		return
	}

	tck := tixer.Ticket{
		ID: tixer.TicketID(id),
	}

	vld := validate.NewValidator()
	if input.Title != nil {
		tck.Title = *input.Title
		tck.ValidateTitle(vld)
	}

	if input.Price != nil {
		tck.Price = *input.Price
		tck.ValidatPrice(vld)
	}

	if !vld.Valid() {
		web.FailedValidationResponse(s.Logger, w, r, vld.Errors)
		return
	}

	tck, err = s.TicketService.UpdateTicket(r.Context(), tck)
	if err != nil {
		switch {
		case errors.Is(err, tixer.ErrTicketNotFound):
			web.NotFoundResponse(s.Logger, w, r)
		default:
			web.ServerErrorResponse(s.Logger, w, r, err)
		}

		return
	}

	err = web.WriteJSON(w, http.StatusOK, web.Envelope{"ticket": mapTicketToResponse(tck)}, nil)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
	}
}

func (s *Server) handleDeleteTicket(w http.ResponseWriter, r *http.Request) {
	id, err := web.ReadIDParam(r)
	if err != nil {
		web.BadRequestResponse(s.Logger, w, r, err)
		return
	}

	err = s.TicketService.DeleteTicket(r.Context(), tixer.TicketID(id))
	if err != nil {
		switch {
		case errors.Is(err, tixer.ErrTicketNotFound):
			web.NotFoundResponse(s.Logger, w, r)
		default:
			web.ServerErrorResponse(s.Logger, w, r, err)
		}

		return
	}

	err = web.WriteJSON(w, http.StatusOK, web.Envelope{"message": "ticket succesfully deleted"}, nil)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
	}
}

func (s *Server) handleReadTickets(w http.ResponseWriter, r *http.Request) {
	vld := validate.NewValidator()

	var input readTickets
	qs := r.URL.Query()
	input.After = web.ReadUUID(qs, "after", uuid.Nil, vld)
	input.Before = web.ReadUUID(qs, "before", uuid.Nil, vld)
	input.Limit = web.ReadInt(qs, "limit", 10, vld)

	if validateReadTickets(vld, input); !vld.Valid() {
		web.FailedValidationResponse(s.Logger, w, r, vld.Errors)
		return
	}

	filter := tixer.Filter{
		After:  tixer.TicketID(input.After),
		Before: tixer.TicketID(input.Before),
		Limit:  input.Limit,
	}

	tt, met, err := s.TicketService.ReadTickets(r.Context(), filter)
	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
		return
	}

	err = web.WriteJSON(w, http.StatusOK, web.Envelope{
		"tickets":    mapTicketListToResponse(tt),
		"pagination": mapMetadataToResponse(met),
	}, nil)

	if err != nil {
		web.ServerErrorResponse(s.Logger, w, r, err)
	}
}

type (
	// createTicket contains the information needed to create a new Ticket.
	createTicket struct {
		Title string  `json:"title"`
		Price float64 `json:"price"`
	}

	// updateTicket contains the information needed to update a Ticket.
	// All fields are optional so clients can send just the fields they want to change.
	// It uses pointer fields so we can differentiate between a field that
	// was not provided and a field that was provided as explicitly blank.
	updateTicket struct {
		Title *string  `json:"title"`
		Price *float64 `json:"price"`
	}

	// readTickets contains the information needed to read a list of Tickets.
	readTickets struct {
		After  uuid.UUID `json:"after"`
		Before uuid.UUID `json:"before"`
		Limit  int       `json:"limit"`
	}
)

type (
	// ticketResponse contains the information about a Ticket that we want to
	// return to clients.
	ticketResponse struct {
		ID    string  `json:"id"`
		Title string  `json:"title"`
		Price float64 `json:"price"`
	}

	// metadataResponse contains the information required to apply pagination
	// on the client side.
	metadataResponse struct {
		After  string `json:"after"`
		Before string `json:"before"`
		Total  int    `json:"total"`
	}
)

// validateReadTickets validates from a 'Presentation' perspective the information
// provided for reading a list of tickets.
func validateReadTickets(vld *validate.Validator, input readTickets) {
	vld.Check(input.Limit > 0 && input.Limit <= 50, "limit", "must be in the interval [0, 50]")
}

func mapTicketToResponse(ticket tixer.Ticket) ticketResponse {
	return ticketResponse{
		ID:    ticket.ID.String(),
		Title: ticket.Title,
		Price: ticket.Price,
	}
}

func mapTicketListToResponse(tickets []tixer.Ticket) []ticketResponse {
	slice := make([]ticketResponse, 0, len(tickets))
	for _, ticket := range tickets {
		slice = append(slice, mapTicketToResponse(ticket))
	}

	return slice
}

func mapMetadataToResponse(m tixer.Metadata) metadataResponse {
	return metadataResponse{
		After:  m.After.String(),
		Before: m.Before.String(),
		Total:  m.Total,
	}
}
