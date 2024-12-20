package main

import (
	"web/config"

	"github.com/gofiber/fiber/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stripe/stripe-go/v80"
)

var cfg = config.LoadConfig()

func main() {
	stripe.Key = cfg.Stripe.Key

	app := fiber.New()

	app.Static("/", "./static")

	app.Get("/", index)
	app.Get("/home", home)
	app.Get("/product", product)
	app.Get("/about", about)
	app.Get("/guide", guide)
	app.Get("/license", license)

	app.Post("/checkout", checkout)
	app.Post("/webhook", webhookHandler)
	app.Get("/landing", landing)
	app.Post("/deliver", deliver)
	app.Get("/onboard", onboardFromEmail)

	// To be implemented
	app.Get("/update/v2/download", AutoUpdate)

	app.Listen(":3000")

}
