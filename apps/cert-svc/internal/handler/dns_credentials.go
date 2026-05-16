package handler

import "github.com/go-chi/chi/v5"

func mountDNSCredentials(r chi.Router) {
	r.Post("/dns-credentials", writeNotImplemented)
	r.Get("/dns-credentials", writeNotImplemented)
	r.Delete("/dns-credentials/{id}", writeNotImplemented)
	r.Post("/dns-credentials/{id}/health-check", writeNotImplemented)
}
