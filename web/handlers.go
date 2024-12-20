package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"reflect"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/checkout/session"
	"github.com/stripe/stripe-go/v80/webhook"
)

func index(c *fiber.Ctx) error {
	return Render(c, HomePage())
}

func home(c *fiber.Ctx) error {
	return Render(c, HomeContent())
}

func product(c *fiber.Ctx) error {
	return Render(c, ProductContent())
}

func about(c *fiber.Ctx) error {
	return Render(c, AboutContent())
}

func guide(c *fiber.Ctx) error {
	return Render(c, GuideContent())
}

func license(c *fiber.Ctx) error {
	return Render(c, LicenseContent())
}

func onboardFromEmail(c *fiber.Ctx) error {
	return Render(c, OnboardPage())
}

func deliver(c *fiber.Ctx) error {
	deliveryEmail := c.FormValue("email")
	if reflect.ValueOf(deliveryEmail).IsZero() {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	sessionId := c.FormValue("session")
	if reflect.ValueOf(sessionId).IsZero() {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	db, err := sql.Open("sqlite3", cfg.DB)
	if err != nil {
		log.Println("Failed to open connection to db:", err)
		return c.SendStatus(fiber.StatusBadGateway)
	}

	defer db.Close()

	var isValid int
	sqlQuery := "SELECT EXISTS (SELECT 1 FROM customer_order WHERE checkout_session = ?)"
	err = db.QueryRow(sqlQuery, sessionId).Scan(&isValid)
	if err != nil || reflect.ValueOf(isValid).IsZero() {
		log.Println("Failed to validate session with db", err)
		return c.SendStatus(fiber.StatusBadGateway)
	}

	onboardingUrl := fmt.Sprintf("%v/onboard", cfg.Domain)
	content := fmt.Sprintf(
		`To: %v
Subject: PAM Solution From JP
MIME-version: 1.0;
Content-Type: text/html; charset="UTF-8";
<html>
	<body>
		<p>	Dear value customer,</p>
		<p>
			Thanks for purchasing our product. 
			Your order is in the attachment section.
			Please follow our onboarding video <a href="%v">here</a> to onboard our product.
			If you encouter any issue, kind contact us at XXXXXXXXXXX.
		</p>
		<p> Best regards,<br>Jason </p>
	</body>
</html>`, deliveryEmail, onboardingUrl)

	go sendMail(content, deliveryEmail, "product")

	return Render(c, DonePage())
}

func checkout(c *fiber.Ctx) error {
	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String("price_1QCiL7FgW7Lx7Qh5FUIwgy3h"),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(cfg.Domain + "/landing?checkout_session={CHECKOUT_SESSION_ID}"),
	}

	s, err := session.New(params)
	if err != nil {
		log.Println("Failed to creaete stripe session:", err)
		return c.SendStatus(fiber.StatusBadGateway)
	}

	return c.Redirect(s.URL)
}

func webhookHandler(c *fiber.Ctx) error {
	body := c.Body()
	signature_header := c.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, signature_header, cfg.Stripe.Endpoint)
	if err != nil {
		fmt.Println("Failed to construct stripe webhook:", err)
		return c.SendStatus(fiber.StatusBadGateway)
	}

	if event.Type == stripe.EventTypeCheckoutSessionCompleted || event.Type == stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded {
		var cs stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &cs)
		if err != nil {
			fmt.Println("Failed to parse webhook json:", err)
			return c.SendStatus(fiber.StatusBadGateway)
		}

		_, err = fulfillment(cs.ID)
		if err != nil {
			log.Println("Failed to perform fulfillment:", err)
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func landing(c *fiber.Ctx) error {
	sessionId := c.FormValue("checkout_session")
	if reflect.ValueOf(sessionId).IsZero() {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	email, err := fulfillment(sessionId)
	if err != nil {
		log.Println("Failed to perform fulfillment:", err)
		return c.SendStatus(fiber.StatusBadRequest)
	}

	return Render(c, LandingPage(email, sessionId))
}

func fulfillment(sessionId string) (string, error) {
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("line_items")

	cs, err := session.Get(sessionId, params)
	if err != nil {
		log.Println("Failed to retreive checkout seesion from stripe:", err)
		return "", err
	}

	db, err := sql.Open("sqlite3", cfg.DB)
	if err != nil {
		log.Println("Failed to open connection to db:", err)
		return "", err
	}

	defer db.Close()

	var status string
	sqlQuery := "SELECT status FROM customer_order WHERE checkout_session = ?"
	err = db.QueryRow(sqlQuery, sessionId).Scan(&status)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Failed to retreive data from db:", err)
		return "", err
	}

	if !reflect.ValueOf(status).IsZero() {
		log.Println("Request for", sessionId, "has been handled")
		return cs.CustomerDetails.Email, nil
	}

	sqlQuery = "INSERT INTO customer_order (checkout_session, email, status) VALUES(?,?,?)"
	_, err = db.Exec(sqlQuery, sessionId, cs.CustomerDetails.Email, "processing")
	if err != nil {
		log.Println("Failed to write data into db:", err)
		return "", err
	}

	if cs.PaymentStatus != stripe.CheckoutSessionPaymentStatusUnpaid {
		go sendInvoice(sessionId, cs.CustomerDetails.Email)
		sqlQuery = "UPDATE customer_order SET status = ? WHERE checkout_session = ?"
		_, err = db.Exec(sqlQuery, "done", sessionId, sessionId)
		if err != nil {
			log.Println("Failed to update status of ", sessionId, "to done:", err)
		}
	}

	return cs.CustomerDetails.Email, nil
}

func sendInvoice(checkoutSession string, customerEmail string) {
	url := fmt.Sprintf("%v/landing?checkout_session=%v", cfg.Domain, checkoutSession)
	content := fmt.Sprintf(
		`To: %v
Subject: Payment Confirmation
MIME-version: 1.0;
Content-Type: text/html; charset="UTF-8";
<html>
	<body>
		<p>	Dear value customer,</p>
		<p>
			Thanks for purchasing our product. 
			Your invoice is in the attachment section.
			If you are not redicted to the checkout page upon successful payment, kindly click <a href="%v">here</a> to continue.
		</p>
		<p> Best regards,<br>Jason </p>
	</body>
</html>`, customerEmail, url)
	sendMail(content, customerEmail, "invoice")
}

func sendMail(content string, customerEmail string, choice string) {
	msg := []byte(content)
	var to = []string{customerEmail}

	auth := smtp.PlainAuth("", cfg.Smtp.Username, cfg.Smtp.Password, cfg.Smtp.Host)
	err := smtp.SendMail(cfg.Smtp.Address, auth, cfg.Smtp.Username, to, msg)
	if err != nil {
		log.Print(err.Error())
	}
	fmt.Println("Sent", choice, "to", customerEmail)
}

// render templ frontend
func Render(c *fiber.Ctx, component templ.Component) error {
	c.Set("Content-Type", "text/html")
	return component.Render(c.Context(), c.Response().BodyWriter())
}

// to be implemented
func AutoUpdate(c *fiber.Ctx) error {
	return c.Download("./testing2.txt", "myfile")
}
