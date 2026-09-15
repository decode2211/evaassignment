// This file implements every endpoint under /tickets: creating a ticket,
// listing "my" tickets, fetching one ticket, and changing a ticket's
// status. All of these routes require the caller to be logged in (see
// internal/auth/middleware.go), and every single one of them is careful to
// only ever touch tickets owned by the logged-in user.
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

// ticketResponse is the JSON shape used for a single ticket everywhere it
// appears in the API (after creating one, in a list, after fetching one by
// ID, and after updating its status) - one consistent shape everywhere.
type ticketResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	UserID      int64  `json:"user_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toTicketResponse converts an internal models.Ticket into the JSON shape
// clients receive.
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

// currentUserID pulls the logged-in user's ID out of the request context.
// It is only ever missing if a route was wired up without going through
// the auth middleware first, which would be a programming mistake rather
// than something a client could trigger - hence the 500 response.
func currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusInternalServerError, "missing authentication context")
		return 0, false
	}
	return id, true
}

// parseTicketID reads the "{id}" path value and ensures it is a plain
// positive integer. A non-numeric ID (like "/tickets/abc") is a client
// mistake, so it is reported as 400 Bad Request rather than 404 - 404 is
// reserved for "well-formed ID, but no such ticket".
func parseTicketID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ticket id must be a positive integer")
	}
	return id, nil
}

// createTicketRequest is the expected JSON body for POST /tickets.
type createTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Status is accepted but intentionally ignored (see CreateTicket) -
	// every new ticket starts as "open" no matter what a client sends.
	Status string `json:"status"`
}

// CreateTicket handles POST /tickets: it creates a new ticket owned by the
// logged-in user.
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

	// Note: we never look at req.Status here. A ticket's starting status
	// is always "open" - allowing a client to create a ticket that is
	// already "in_progress" or "closed" would make no sense and would
	// bypass the whole point of having a status workflow.
	ticket, err := h.Store.CreateTicket(r.Context(), userID, title, description)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not create ticket")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toTicketResponse(ticket))
}

// ListTickets handles GET /tickets: it returns every ticket owned by the
// logged-in user, newest first, and nothing belonging to anyone else.
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

	// Build the response slice explicitly (rather than returning a nil
	// slice through json.Marshal) so an empty result is "[]", never
	// "null" - a client's JSON parser can always safely loop over "[]",
	// but "null" often needs special-casing.
	out := make([]ticketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, toTicketResponse(t))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// GetTicket handles GET /tickets/{id}: it returns one ticket, but only if
// the logged-in user owns it.
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
		// Whether the ticket truly does not exist, or it exists but
		// belongs to a different user, we return the exact same 404
		// response either way. If we returned 403 Forbidden for "exists
		// but not yours", a user could probe ticket IDs and learn which
		// ones belong to someone else just from the status code - 404
		// keeps other users' data invisible, not just unreadable.
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "ticket not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not fetch ticket")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toTicketResponse(ticket))
}

// updateStatusRequest is the expected JSON body for
// PATCH /tickets/{id}/status.
type updateStatusRequest struct {
	Status string `json:"status"`
}

// UpdateTicketStatus handles PATCH /tickets/{id}/status: it moves a ticket
// forward in its lifecycle (open -> in_progress -> closed), but only for
// tickets the logged-in user owns, and only along the allowed paths
// defined in internal/models.CanTransition.
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
		// Same reasoning as GetTicket: a ticket that belongs to someone
		// else is reported as "not found", not "forbidden".
		httpx.WriteError(w, http.StatusNotFound, "ticket not found")
	default:
		// Any other error here comes from models.CanTransition (an
		// invalid or backward status change, or "already has this
		// status") - all of those are the client's mistake, so 400.
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	}
}
