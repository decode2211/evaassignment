// Endpoints under /tickets: create, list, get, and update status.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/httpx"
	"github.com/decode2211/evaassignment/internal/models"
	"github.com/decode2211/evaassignment/internal/store"
)

// ticketResponse is the JSON shape used everywhere a ticket is returned.
type ticketResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	UserID      int64  `json:"user_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toTicketResponse(t models.Ticket) ticketResponse {
	return ticketResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		UserID:      t.UserID,
		CreatedAt:   t.CreatedAt.Format(rfc3339),
		UpdatedAt:   t.UpdatedAt.Format(rfc3339),
	}
}

// currentUserID reads the logged-in user's ID set by the auth middleware.
func currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusInternalServerError, "missing authentication context")
		return 0, false
	}
	return id, true
}

// parseTicketID reads the "{id}" path value as a positive integer.
// A non-numeric ID is a client mistake (400), not a missing-ticket case (404).
func parseTicketID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ticket id must be a positive integer")
	}
	return id, nil
}

// createTicketRequest is the JSON body for POST /tickets.
type createTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // accepted but ignored, see CreateTicket
}

// CreateTicket handles POST /tickets.
func (h *Handlers) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	var req createTicketRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(title) > maxTitleLength {
		httpx.WriteError(w, http.StatusBadRequest, "title must be at most 200 characters")
		return
	}
	description := strings.TrimSpace(req.Description)

	// req.Status is ignored - every new ticket starts as open.
	ticket, err := h.Store.CreateTicket(r.Context(), userID, title, description)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not create ticket")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toTicketResponse(ticket))
}

// ListTickets handles GET /tickets.
func (h *Handlers) ListTickets(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	tickets, err := h.Store.ListTicketsByUser(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list tickets")
		return
	}

	// Built explicitly so an empty result serializes as [], not null.
	out := make([]ticketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, toTicketResponse(t))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// GetTicket handles GET /tickets/{id}.
func (h *Handlers) GetTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	id, err := parseTicketID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	ticket, err := h.Store.GetTicketByIDForUser(r.Context(), id, userID)
	if err != nil {
		// 404, not 403, for another user's ticket - so a client can't tell
		// the difference between "not yours" and "doesn't exist".
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "ticket not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not fetch ticket")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toTicketResponse(ticket))
}

// updateStatusRequest is the JSON body for PATCH /tickets/{id}/status.
type updateStatusRequest struct {
	Status string `json:"status"`
}

// UpdateTicketStatus handles PATCH /tickets/{id}/status.
func (h *Handlers) UpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	id, err := parseTicketID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req updateStatusRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	newStatus := models.Status(strings.TrimSpace(req.Status))
	if !newStatus.IsValid() {
		httpx.WriteError(w, http.StatusBadRequest, "status must be one of: open, in_progress, closed")
		return
	}

	ticket, err := h.Store.UpdateTicketStatus(r.Context(), id, userID, newStatus)
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, toTicketResponse(ticket))
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "ticket not found")
	default:
		// Anything else comes from models.CanTransition - a bad or
		// backward status change, which is the client's mistake.
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	}
}
