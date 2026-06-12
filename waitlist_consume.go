package lib

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/taubyte/go-sdk/database"
	"github.com/taubyte/go-sdk/event"
)

type consumeBody struct {
	Email string `json:"email"`
}

//export apiWaitlistConsume
func apiWaitlistConsume(e event.Event) uint32 {
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
	body, _ := io.ReadAll(h.Body())
	defer h.Body().Close()
	var req consumeBody
	_ = json.Unmarshal(body, &req)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		return wlJSONError(h, 400, "email requis")
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
	idx := findIdx(list, "", email)
	if idx < 0 {
		return wlJSONError(h, 403, "accès non autorisé")
	}
	e2 := &list[idx]
	if e2.AccessDisabled || !e2.AccessEnabled {
		return wlJSONError(h, 403, "accès beta désactivé")
	}
	if e2.AiGenerationsGrant <= 0 || e2.AiGenerationsUsed >= e2.AiGenerationsGrant {
		return wlJSONError(h, 403, "quota épuisé")
	}
	e2.AiGenerationsUsed++
	_ = writeWaitlist(db, list)
	rem := e2.AiGenerationsGrant - e2.AiGenerationsUsed
	return wlJSONOK(h, 200, map[string]interface{}{
		"active": true, "status": "active", "total": e2.AiGenerationsGrant,
		"used": e2.AiGenerationsUsed, "remaining": rem, "email": e2.Email,
	})
}
