package lib

import (
	"strings"

	"github.com/taubyte/go-sdk/database"
	"github.com/taubyte/go-sdk/event"
)

//export apiWaitlistAccess
func apiWaitlistAccess(e event.Event) uint32 {
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
	email, _ := h.Query().Get("email")
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return wlJSONError(h, 400, "email requis")
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return wlJSONError(h, 500, "base indisponible")
	}
	defer db.Close()
	list, _ := readWaitlist(db)
	for i := range list {
		if strings.EqualFold(strings.TrimSpace(list[i].Email), email) {
			e := &list[i]
			status := "pending"
			active := false
			if e.AccessDisabled {
				status = "disabled"
			} else if e.AccessEnabled || e.AiGenerationsGrant > 0 {
				status, active = "active", true
			}
			total, used := e.AiGenerationsGrant, e.AiGenerationsUsed
			if total < 0 {
				total = 0
			}
			if used < 0 {
				used = 0
			}
			rem := total - used
			if rem < 0 {
				rem = 0
			}
			if active && rem <= 0 && total > 0 {
				active, status = false, "exhausted"
			}
			return wlJSONOK(h, 200, map[string]interface{}{
				"active": active, "status": status, "total": total, "used": used,
				"remaining": rem, "email": e.Email,
			})
		}
	}
	return wlJSONOK(h, 200, map[string]interface{}{"active": false, "status": "none", "total": 0, "used": 0, "remaining": 0})
}
