package handler

import "github.com/go-chi/chi/v5"

func mountOrders(r chi.Router) {
	r.Post("/orders", writeNotImplemented)
	r.Get("/orders", writeNotImplemented)
	r.Get("/orders/{id}", writeNotImplemented)
	r.Post("/orders/{id}/retry", writeNotImplemented)
}
