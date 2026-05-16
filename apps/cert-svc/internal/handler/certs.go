package handler

import "github.com/go-chi/chi/v5"

func mountCerts(r chi.Router) {
	r.Get("/certs", writeNotImplemented)
	r.Get("/certs/{id}", writeNotImplemented)
	r.Post("/certs/{id}/download", writeNotImplemented)
	r.Post("/certs/{id}/revoke", writeNotImplemented)
}
