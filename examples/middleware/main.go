package main

import (
	"fmt"
	"log"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

func main() {
	root := router.New()

	root.Before(
		router.MiddlewareFunc(
			func(ctx *fasthttp.RequestCtx) {
				println("BeforeRequestHandler!")
			},
		),
	)

	root.After(
		router.MiddlewareFunc(
			func(ctx *fasthttp.RequestCtx) {
				println("AfterRequestHandler!")
			},
		),
	)

	root.GET(
		"/",
		func(ctx *fasthttp.RequestCtx) {
			println("RequestHandler!")
			fmt.Fprintf(ctx, "Hello World!\n")
		},
	)

	log.Fatal(fasthttp.ListenAndServe(":8080", root.Handler))
}
