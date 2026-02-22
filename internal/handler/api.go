package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"reminderin/internal/store"
	"reminderin/internal/whatsapp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/skip2/go-qrcode"
)

const (
	maxJSONBodyBytes int64 = 1 << 20
	maxMessageChars        = 4000
)

var reminderCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

var (
	waDirectNumberPattern = regexp.MustCompile(`^\d{6,15}$`)
	waGroupPattern        = regexp.MustCompile(`^\d+-\d+(@g\.us)?$`)
	waJIDPattern          = regexp.MustCompile(`^\d+@(s\.whatsapp\.net|g\.us|broadcast)$`)
)

type APIHandler struct {
	Store       *store.SQLiteStore
	WaMgr       *whatsapp.ClientManager
	LinkLimiter chan struct{}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *APIHandler) CreateReminder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message     string `json:"message"`
		TargetWa    string `json:"target_wa"`
		Recurrence  string `json:"recurrence"`
		ScheduledAt string `json:"scheduled_at"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	req.TargetWa = strings.TrimSpace(req.TargetWa)
	req.Recurrence = strings.TrimSpace(req.Recurrence)

	normalizedTargets, err := normalizeReminderTargets(req.TargetWa)
	if err != nil {
		http.Error(w, "Invalid target format", http.StatusBadRequest)
		return
	}
	req.TargetWa = normalizedTargets

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}
	if len([]rune(req.Message)) > maxMessageChars {
		http.Error(w, "Message is too long", http.StatusBadRequest)
		return
	}

	parsedTime, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		http.Error(w, "Invalid time format (use RFC3339)", http.StatusBadRequest)
		return
	}

	target := req.TargetWa
	if target == "" {
		target = h.Store.GetWANumber()
	}

	rem := store.Reminder{
		ID:         uuid.Must(uuid.NewV7()),
		Message:    req.Message,
		TargetWa:   target,
		Recurrence: req.Recurrence,
		IsActive:   true,
	}

	now := time.Now()
	if req.Recurrence == "" {
		if !parsedTime.After(now) {
			http.Error(w, "Scheduled time must be in the future", http.StatusBadRequest)
			return
		}
		rem.ScheduledAt = parsedTime
	} else if strings.HasPrefix(req.Recurrence, "plugin:") {
		http.Error(w, "Plugin recurrence is not supported", http.StatusBadRequest)
		return
	} else {
		nextRun, err := nextScheduledTime(req.Recurrence, parsedTime)
		if err != nil {
			http.Error(w, "Invalid cron expression", http.StatusBadRequest)
			return
		}
		rem.ScheduledAt = nextRun
	}

	if err := h.Store.CreateReminder(rem); err != nil {
		http.Error(w, "Failed to create reminder", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, rem)
}

func (h *APIHandler) ListReminders(w http.ResponseWriter, r *http.Request) {
	cursorStr := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("order")
	search := r.URL.Query().Get("search")

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	var cursor *uuid.UUID
	if cursorStr != "" {
		parsed, err := uuid.Parse(cursorStr)
		if err == nil {
			cursor = &parsed
		}
	}

	etag := remindersListETag(h.Store.Version(), cursorStr, limit, search, sortBy, sortOrder)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	rems, nextCursor, total := h.Store.GetReminders(cursor, limit, search, sortBy, sortOrder)

	w.Header().Set("ETag", etag)

	response := map[string]interface{}{
		"data":        rems,
		"next_cursor": nextCursor,
		"total":       total,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *APIHandler) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.Store.DeleteReminder(id)
	if err != nil {
		if errors.Is(err, store.ErrReminderNotFound) {
			http.Error(w, "Reminder not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete reminder", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) DeleteAllReminders(w http.ResponseWriter, r *http.Request) {
	err := h.Store.DeleteAllReminders()
	if err != nil {
		http.Error(w, "Failed to delete reminders", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req store.Reminder
	if !decodeJSONBody(w, r, &req) {
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	req.TargetWa = strings.TrimSpace(req.TargetWa)
	req.Recurrence = strings.TrimSpace(req.Recurrence)

	normalizedTargets, targetErr := normalizeReminderTargets(req.TargetWa)
	if targetErr != nil {
		http.Error(w, "Invalid target format", http.StatusBadRequest)
		return
	}
	req.TargetWa = normalizedTargets

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}
	if len([]rune(req.Message)) > maxMessageChars {
		http.Error(w, "Message is too long", http.StatusBadRequest)
		return
	}

	now := time.Now()
	if req.Recurrence == "" {
		if !req.ScheduledAt.After(now) {
			http.Error(w, "Scheduled time must be in the future", http.StatusBadRequest)
			return
		}
	} else if strings.HasPrefix(req.Recurrence, "plugin:") {
		http.Error(w, "Plugin recurrence is not supported", http.StatusBadRequest)
		return
	} else {
		nextRun, err := nextScheduledTime(req.Recurrence, req.ScheduledAt)
		if err != nil {
			http.Error(w, "Invalid cron expression", http.StatusBadRequest)
			return
		}
		req.ScheduledAt = nextRun
	}

	err = h.Store.UpdateReminder(id, req)
	if err != nil {
		if errors.Is(err, store.ErrReminderNotFound) {
			http.Error(w, "Reminder not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to update reminder", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *APIHandler) ToggleReminder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.Store.ToggleReminderActive(id)
	if err != nil {
		if errors.Is(err, store.ErrReminderNotFound) {
			http.Error(w, "Reminder not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to toggle reminder", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *APIHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	waNumber := h.Store.GetWANumber()
	if waNumber == "" {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	groups, err := h.WaMgr.GetJoinedGroups(waNumber)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (h *APIHandler) ListContacts(w http.ResponseWriter, r *http.Request) {
	waNumber := h.Store.GetWANumber()
	if waNumber == "" {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	contacts, err := h.WaMgr.GetContacts(waNumber)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	writeJSON(w, http.StatusOK, contacts)
}

func (h *APIHandler) GetWAStatus(w http.ResponseWriter, r *http.Request) {
	waNumber := h.Store.GetWANumber()
	if waNumber == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "not_linked"})
		return
	}

	client, err := h.WaMgr.GetClient(waNumber)
	if err != nil || client == nil || !client.IsConnected() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected", "number": waNumber})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "number": waNumber})
}

func (h *APIHandler) UnlinkWA(w http.ResponseWriter, r *http.Request) {
	waNumber := h.Store.GetWANumber()
	if waNumber == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not linked"})
		return
	}

	_ = h.WaMgr.Logout(waNumber)
	if err := h.Store.UpdateWANumber(""); err != nil {
		http.Error(w, "failed to unlink whatsapp", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) GetQR(w http.ResponseWriter, r *http.Request) {
	if !h.acquireLinkSlot() {
		http.Error(w, "Too many active link sessions", http.StatusTooManyRequests)
		return
	}
	defer h.releaseLinkSlot()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := h.WaMgr.GetNewAuthClient()
	linked := false
	defer func() {
		if !linked {
			client.Disconnect()
		}
	}()

	qrChan, err := h.WaMgr.GetLinkQR(client)
	if err != nil {
		sendSSE(w, flusher, map[string]string{"type": "error", "message": err.Error()})
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-qrChan:
			if !ok {
				return
			}
			switch evt.Event {
			case "code":
				qrImage, err := qrCodeDataURI(evt.Code)
				if err != nil {
					sendSSE(w, flusher, map[string]string{"type": "error", "message": "failed to render qr"})
					return
				}
				sendSSE(w, flusher, map[string]string{"type": "qr", "image": qrImage, "code": evt.Code})
			case "success":
				if client.Store.ID == nil {
					sendSSE(w, flusher, map[string]string{"type": "error", "message": "missing linked account"})
					return
				}
				waNumber := client.Store.ID.User
				if err := h.Store.UpdateWANumber(waNumber); err != nil {
					sendSSE(w, flusher, map[string]string{"type": "error", "message": "failed to save linked number"})
					return
				}
				h.WaMgr.AddClient(client)
				linked = true
				sendSSE(w, flusher, map[string]string{"type": "success", "number": waNumber})
				return
			case "error", "timeout":
				sendSSE(w, flusher, map[string]string{"type": "error", "message": evt.Event})
				return
			}
		}
	}
}

func (h *APIHandler) GetPairCode(w http.ResponseWriter, r *http.Request) {
	if !h.acquireLinkSlot() {
		http.Error(w, "Too many active link sessions", http.StatusTooManyRequests)
		return
	}
	defer h.releaseLinkSlot()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	phone := normalizePhone(r.URL.Query().Get("phone"))
	if len(phone) < 8 || len(phone) > 20 {
		sendSSE(w, flusher, map[string]string{"type": "error", "message": "Phone number required"})
		return
	}

	client := h.WaMgr.GetNewAuthClient()
	linked := false
	defer func() {
		if !linked {
			client.Disconnect()
		}
	}()

	ch, err := client.GetQRChannel(r.Context())
	if err != nil {
		sendSSE(w, flusher, map[string]string{"type": "error", "message": err.Error()})
		return
	}

	code, err := h.WaMgr.GetLinkCode(client, phone)
	if err != nil {
		sendSSE(w, flusher, map[string]string{"type": "error", "message": err.Error()})
		return
	}

	sendSSE(w, flusher, map[string]string{"type": "code", "code": code})

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.Event == "success" {
				if client.Store.ID == nil {
					sendSSE(w, flusher, map[string]string{"type": "error", "message": "missing linked account"})
					return
				}
				waNumber := client.Store.ID.User
				if err := h.Store.UpdateWANumber(waNumber); err != nil {
					sendSSE(w, flusher, map[string]string{"type": "error", "message": "failed to save linked number"})
					return
				}
				h.WaMgr.AddClient(client)
				linked = true
				sendSSE(w, flusher, map[string]string{"type": "success", "number": waNumber})
				return
			}
			if evt.Event == "error" || evt.Event == "timeout" {
				sendSSE(w, flusher, map[string]string{"type": "error", "message": evt.Event})
				return
			}
		}
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return false
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

func nextScheduledTime(recurrence string, requested time.Time) (time.Time, error) {
	sched, err := reminderCronParser.Parse(recurrence)
	if err != nil {
		return time.Time{}, err
	}
	base := time.Now()
	if requested.After(base) {

		if sched.Next(requested.Add(-time.Second)).Equal(requested) {
			return requested, nil
		}
		base = requested
	}
	return sched.Next(base), nil
}

func normalizePhone(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeReminderTargets(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	targets := store.ParseTargets(input)
	if len(targets) == 0 {
		return "", nil
	}
	for _, target := range targets {
		if !isValidReminderTarget(target) {
			return "", fmt.Errorf("invalid target %q", target)
		}
	}

	return strings.Join(targets, ","), nil
}

func isValidReminderTarget(target string) bool {
	if waDirectNumberPattern.MatchString(target) {
		return true
	}
	if waGroupPattern.MatchString(target) {
		return true
	}
	if waJIDPattern.MatchString(target) {
		return true
	}
	return false
}

func qrCodeDataURI(code string) (string, error) {
	pngBytes, err := qrcode.Encode(code, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), nil
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"type":"error","message":"internal error"}`)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
	flusher.Flush()
}

func remindersListETag(version uint64, cursor string, limit int, search, sortBy, sortOrder string) string {
	raw := fmt.Sprintf(
		"v=%d|c=%s|l=%d|q=%s|s=%s|o=%s",
		version,
		cursor,
		limit,
		search,
		sortBy,
		sortOrder,
	)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf(`"r%x"`, sum[:8])
}

func (h *APIHandler) acquireLinkSlot() bool {
	if h == nil || h.LinkLimiter == nil {
		return true
	}
	select {
	case h.LinkLimiter <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *APIHandler) releaseLinkSlot() {
	if h == nil || h.LinkLimiter == nil {
		return
	}
	select {
	case <-h.LinkLimiter:
	default:
	}
}
