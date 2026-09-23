package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/customer"
	"github.com/stripe/stripe-go/v86/invoice"
	"github.com/stripe/stripe-go/v86/invoiceitem"
	"github.com/stripe/stripe-go/v86/webhook"
	"github.com/xolra0d/novikontas-register-page/shared/pkg/api"
)

type Handles struct {
	database *Database
	logger   *slog.Logger

	HomeTemplate          *template.Template
	CourseDetailsTemplate *template.Template
	publicURL             string
	stripeWebhookSecret   string
}

func NewHandles(database *Database, logger *slog.Logger, homeTemplate *template.Template, courseDetailsTemplate *template.Template, publicURL, stripeWebhookSecret string) *Handles {
	return &Handles{
		database: database, logger: logger, HomeTemplate: homeTemplate,
		CourseDetailsTemplate: courseDetailsTemplate, publicURL: strings.TrimRight(publicURL, "/"),
		stripeWebhookSecret: stripeWebhookSecret,
	}
}

// Ping handles /ok requests
func (h *Handles) Ping(w http.ResponseWriter, _ *http.Request) {
	const op = "main.Ping"

	err := api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

// ListCourses handles / requests
func (h *Handles) ListCourses(w http.ResponseWriter, r *http.Request) {
	const op = "main.ListCourses"

	courses, err := h.database.GetCourses(r.Context())
	if err != nil {
		h.logger.Error("Failed to get courses", "error", err)
		return
	}

	config := HomeTemplateConfig{"Demo", courses}
	err = h.HomeTemplate.Execute(w, &config)
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

func (h *Handles) CourseDetails(w http.ResponseWriter, r *http.Request) {
	const op = "main.ListCourses"

	idS := r.PathValue("id")
	id, err := strconv.ParseInt(idS, 10, 64)
	if err != nil {
		panic(err)
	}
	course, err := h.database.GetCourse(r.Context(), id)
	if err != nil {
		panic("NOO1")
	}

	config := CourseDetailsConfig{"T", course}
	err = h.CourseDetailsTemplate.Execute(w, &config)
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

func (h *Handles) EnrollmentForm(w http.ResponseWriter, r *http.Request) {
	const op = "main.EnrollmentForm"

	course, err := h.courseFromRequest(r)
	if err != nil {
		http.Error(w, "course not found", http.StatusNotFound)
		return
	}

	err = h.CourseDetailsTemplate.ExecuteTemplate(w, "enrollment-modal", &CourseDetailsConfig{
		Title:  "Enrollment",
		Course: course,
	})
	if err != nil {
		h.logger.Error("could not write enrollment form", "op", op, "error", err)
	}
}

func (h *Handles) SubmitEnrollment(w http.ResponseWriter, r *http.Request) {
	const op = "main.SubmitEnrollment"

	if err := r.ParseForm(); err != nil {
		http.Error(w, "could not read enrollment form", http.StatusBadRequest)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	paymentType := r.FormValue("payment_type")
	startDate := strings.TrimSpace(r.FormValue("start_date"))
	if fullName == "" || email == "" || paymentType == "" || startDate == "" ||
		(paymentType != "invoice" && paymentType != "pay_now") {
		http.Error(w, "Please provide a full name, email, and payment type.", http.StatusBadRequest)
		return
	}

	course, err := h.courseFromRequest(r)
	if err != nil || !course.Available {
		http.Error(w, "This course is not available.", http.StatusConflict)
		return
	}

	metadata := map[string]string{
		"course_id":  strconv.FormatInt(course.ID, 10),
		"start_date": startDate,
		"full_name":  fullName,
	}
	if phone := strings.TrimSpace(r.FormValue("phone")); phone != "" {
		metadata["phone"] = phone
	}

	if paymentType == "pay_now" {
		checkout, createErr := h.createCheckoutSession(course, email, metadata)
		if createErr != nil {
			h.logger.Error("could not create Stripe Checkout Session", "op", op, "error", createErr)
			http.Error(w, "Could not start payment.", http.StatusBadGateway)
			return
		}
		w.Header().Set("HX-Redirect", checkout.URL)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	sentInvoice, createErr := h.createInvoice(course, fullName, email, metadata)
	if createErr != nil {
		h.logger.Error("could not create Stripe invoice", "op", op, "error", createErr)
		http.Error(w, "Could not create invoice.", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, err = w.Write([]byte(`<div class="notification is-success is-light">The invoice was sent to your email.</div>`))
	if err != nil {
		h.logger.Error("could not write enrollment result", "op", op, "error", err)
	}
	_ = sentInvoice
}

func (h *Handles) createCheckoutSession(course Course, email string, metadata map[string]string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		CustomerEmail:     stripe.String(email),
		SuccessURL:        stripe.String(h.publicURL + "/course/" + strconv.FormatInt(course.ID, 10) + "?enrollment=success"),
		CancelURL:         stripe.String(h.publicURL + "/course/" + strconv.FormatInt(course.ID, 10) + "?enrollment=cancelled"),
		ClientReferenceID: stripe.String("course-" + strconv.FormatInt(course.ID, 10) + "-" + metadata["start_date"]),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(string(stripe.CurrencyEUR)),
				UnitAmount: stripe.Int64(course.Price),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(course.Name),
				},
			},
			Quantity: stripe.Int64(1),
		}},
		Metadata: metadata,
	}
	return session.New(params)
}

func (h *Handles) createInvoice(course Course, fullName, email string, metadata map[string]string) (*stripe.Invoice, error) {
	customerParams := &stripe.CustomerParams{
		Name:     stripe.String(fullName),
		Email:    stripe.String(email),
		Metadata: metadata,
	}
	newCustomer, err := customer.New(customerParams)
	if err != nil {
		return nil, err
	}

	_, err = invoiceitem.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(newCustomer.ID),
		Amount:      stripe.Int64(course.Price),
		Currency:    stripe.String(string(stripe.CurrencyEUR)),
		Description: stripe.String(course.Name + " - start date " + metadata["start_date"]),
		Metadata:    metadata,
	})
	if err != nil {
		return nil, err
	}

	sentInvoice, err := invoice.New(&stripe.InvoiceParams{
		Customer:                    stripe.String(newCustomer.ID),
		CollectionMethod:            stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:                stripe.Int64(7),
		PendingInvoiceItemsBehavior: stripe.String("include"),
		Description:                 stripe.String(course.Name),
		Metadata:                    metadata,
	})
	if err != nil {
		return nil, err
	}
	return invoice.SendInvoice(sentInvoice.ID, &stripe.InvoiceSendInvoiceParams{})
}

func (h *Handles) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	const op = "main.StripeWebhook"

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid webhook body", http.StatusBadRequest)
		return
	}
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.stripeWebhookSecret)
	if err != nil {
		http.Error(w, "invalid webhook signature", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted, stripe.EventTypeInvoicePaid:
		h.logger.Info("Stripe payment completed", "op", op, "event_id", event.ID, "event_type", event.Type)
	case stripe.EventTypeCheckoutSessionExpired, stripe.EventTypeInvoicePaymentFailed:
		h.logger.Info("Stripe payment requires attention", "op", op, "event_id", event.ID, "event_type", event.Type)
	}

	if event.Data != nil && event.Data.Raw != nil {
		var object map[string]any
		if err := json.Unmarshal(event.Data.Raw, &object); err != nil {
			h.logger.Error("could not decode Stripe webhook object", "op", op, "error", err)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handles) courseFromRequest(r *http.Request) (Course, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return Course{}, err
	}
	return h.database.GetCourse(r.Context(), id)
}
