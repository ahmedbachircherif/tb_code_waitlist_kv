package lib

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/taubyte/go-sdk/database"
	"github.com/taubyte/go-sdk/event"
)

type deleteBody struct {
	ID, Email string
}

//export apiWaitlistDelete
func apiWaitlistDelete(e event.Event) uint32 {
	h, err := e.HTTP()
	if err != nil {
		return 1
	}
	method, _ := h.Method()
	if strings.ToUpper(strings.TrimSpace(method)) == "OPTIONS" {
		setWaitlistCors(h)
		h.Return(204)
		return 0
	}
	if !adminKeyOK(h) {
		return wlJSONError(h, 401, "clé admin waitlist requise")
	}
	var req deleteBody
	body, _ := io.ReadAll(h.Body())
	if len(body) > 0 {
		_ = json.Unmarshal(body, &req)
	}
	if req.ID == "" {
		if id, e := h.Query().Get("id"); e == nil {
			req.ID = id
		}
	}
	if req.Email == "" {
		if em, e := h.Query().Get("email"); e == nil {
			req.Email = em
		}
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return wlJSONError(h, 500, "base indisponible")
	}
	defer db.Close()
	list, err := readWaitlist(db)
	if err != nil {
		return wlJSONError(h, 500, "lecture waitlist")
	}
	idx := findIdx(list, req.ID, req.Email)
	if idx < 0 {
		return wlJSONError(h, 404, "inscription introuvable")
	}
	removed := list[idx]
	list = append(list[:idx], list[idx+1:]...)
	_ = writeWaitlist(db, list)
	return wlJSONOK(h, 200, map[string]interface{}{"ok": true, "deleted": true, "email": removed.Email})
}
