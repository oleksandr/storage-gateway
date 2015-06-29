package main

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/julienschmidt/httprouter"
)

// "The problem with httprouter is its non-compatibility with http.Handler.
// But we can make it compatible with a little glue code with our existing
// middlewares and contexts. To do this we wrap our middleware stack –
// implementing http.Handler – into a httprouter.Handler function."
// (c) http://nicolasmerouze.com/guide-routers-golang/
type router struct {
	*httprouter.Router
}

func newRouter() *router {
	return &router{httprouter.New()}
}

func (r *router) get(path string, handler http.Handler) {
	r.GET(path, wrapHandler(handler))
}

func (r *router) post(path string, handler http.Handler) {
	r.POST(path, wrapHandler(handler))
}

func (r *router) put(path string, handler http.Handler) {
	r.PUT(path, wrapHandler(handler))
}

func (r *router) delete(path string, handler http.Handler) {
	r.DELETE(path, wrapHandler(handler))
}

func (r *router) patch(path string, handler http.Handler) {
	r.PATCH(path, wrapHandler(handler))
}

func (r *router) head(path string, handler http.Handler) {
	r.HEAD(path, wrapHandler(handler))
}

func wrapHandler(h http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		context.Set(r, CtxParamsKey, ps)
		h.ServeHTTP(w, r)
	}
}
