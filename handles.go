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

	publicURL           string
	stripeWebhookSecret string

	HomeTemplate          *template.Template
	CourseDetailsTemplate *template.Template

	sheets *Sheets
}

// NewHandles creates new HTTP handles.
func NewHandles(database *Database, logger *slog.Logger, homeTemplate *template.Template, courseDetailsTemplate *template.Template, publicURL, stripeWebhookSecret string, sheets *Sheets) *Handles {
	return &Handles{
		database: database, logger: logger, HomeTemplate: homeTemplate,
		CourseDetailsTemplate: courseDetailsTemplate, publicURL: strings.TrimRight(publicURL, "/"),
		stripeWebhookSecret: stripeWebhookSecret, sheets: sheets,
	}
}

// Ping handles /ok requests
func (h *Handles) Ping(w http.ResponseWriter, _ *http.Request) {
	const op = "handles.Ping"

	err := api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

// ListCourses handles / requests
func (h *Handles) ListCourses(w http.ResponseWriter, r *http.Request) {
	const op = "handles.ListCourses"

	courses, err := h.database.GetCourses(r.Context())
	if err != nil {
		h.logger.Error("Failed to get courses", "error", err, "op", op)
		http.Error(w, "Failed to receive courses", http.StatusInternalServerError)
		return
	}

	config := HomeTemplateConfig{"Courses list", courses}
	err = h.HomeTemplate.Execute(w, &config)
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

func (h *Handles) CourseDetails(w http.ResponseWriter, r *http.Request) {
	const op = "handles.CourseDetails"

	course, err := h.courseFromRequest(r)
	if err != nil {
		h.logger.Error("could not find course", "op", op, "error", err)
		http.NotFound(w, r)
		return
	}

	config := CourseDetailsConfig{
		Title:          course.Name,
		Course:         course,
		PaymentSuccess: r.URL.Query().Get("sign-up") == "success",
	}
	err = h.CourseDetailsTemplate.Execute(w, &config)
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

func (h *Handles) SignUpForm(w http.ResponseWriter, r *http.Request) {
	const op = "handles.SignUpForm"

	course, err := h.courseFromRequest(r)
	if err != nil {
		h.logger.Error("could not find course", "op", op, "error", err)
		http.NotFound(w, r)
		return
	}

	err = h.CourseDetailsTemplate.ExecuteTemplate(w, "sign-up-modal", &CourseDetailsConfig{
		Title:  "Sign Up",
		Course: course,
	})
	if err != nil {
		h.logger.Error("could not write response", "op", op, "error", err)
		return
	}
}

func (h *Handles) SubmitSignUp(w http.ResponseWriter, r *http.Request) {
	const op = "handles.SubmitSignUp"

	if err := r.ParseForm(); err != nil {
		http.Error(w, "could not read form", http.StatusBadRequest)
		h.logger.Error("could not read form", "op", op, "error", err)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	paymentType := r.FormValue("payment_type")
	startDate := strings.TrimSpace(r.FormValue("start_date"))
	phone := strings.TrimSpace(r.FormValue("phone"))

	if fullName == "" || email == "" || paymentType == "" || startDate == "" ||
		(paymentType != "invoice" && paymentType != "pay_now") {
		http.Error(w, "Please provide a full name, email, and payment type.", http.StatusBadRequest)
		h.logger.Error("invalid form", "op", op)
		return
	}

	course, err := h.courseFromRequest(r)
	if err != nil || !course.Available {
		http.Error(w, "This course is not available.", http.StatusConflict)
		return
	}

	metadata := map[string]string{
		"course_id":   strconv.FormatInt(course.ID, 10),
		"course_name": course.Name,
		"start_date":  startDate,
		"full_name":   fullName,
		"email":       email,
	}
	if phone != "" {
		metadata["phone"] = phone
	}

	if paymentType == "pay_now" {
		checkout, err := h.createCheckoutSession(course, email, metadata)
		if err != nil {
			h.logger.Error("could not create session", "op", op, "error", err)
			http.Error(w, "Could not initiate payment.", http.StatusBadGateway)
			return
		}
		w.Header().Set("HX-Redirect", checkout.URL)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	_, err = h.createInvoice(course, fullName, email, metadata)
	if err != nil {
		h.logger.Error("could not initiate invoice", "op", op, "error", err)
		http.Error(w, "Could not initiate invoice.", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, err = w.Write([]byte(`<div class="notification is-success is-light">The invoice was sent to your email.</div>`))
	if err != nil {
		h.logger.Error("could not write sign-up result", "op", op, "error", err)
	}
}

func (h *Handles) createCheckoutSession(course Course, email string, metadata map[string]string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		CustomerEmail:     stripe.String(email),
		SuccessURL:        stripe.String(h.publicURL + "/course/" + strconv.FormatInt(course.ID, 10) + "?sign-up=success"),
		CancelURL:         stripe.String(h.publicURL + "/course/" + strconv.FormatInt(course.ID, 10) + "?sign-up=cancelled"),
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
	const op = "handles.createInvoice"

	customerParams := &stripe.CustomerParams{
		Name:     stripe.String(fullName),
		Email:    stripe.String(email),
		Metadata: metadata,
	}
	newCustomer, err := customer.New(customerParams)
	if err != nil {
		h.logger.Error("failed to create new customer", "op", op, "error", err)
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
		h.logger.Error("failed to create new item", "op", op, "error", err)
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
		h.logger.Error("failed to create new invoice", "op", op, "error", err)
		return nil, err
	}
	return invoice.SendInvoice(sentInvoice.ID, &stripe.InvoiceSendInvoiceParams{})
}

func (h *Handles) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	const op = "handles.StripeWebhook"

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
	case stripe.EventTypeCheckoutSessionCompleted:
		var checkout stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &checkout); err != nil {
			h.logger.Error("could not decode completed session", "op", op, "error", err)
			http.Error(w, "invalid webhook object", http.StatusBadRequest)
			return
		}
		if checkout.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
			payment := paymentInfoFromMetadata(checkout.Metadata)
			if err := h.sheets.AppendRecord(r.Context(), payment); err != nil {
				h.logger.Error("could not append paid checkout to Google Sheet", "op", op, "error", err)
				http.Error(w, "could not record payment", http.StatusInternalServerError)
				return
			}
		}
		h.logger.Info("Stripe payment completed", "op", op, "event_id", event.ID, "event_type", event.Type)
	case stripe.EventTypeInvoicePaid:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			h.logger.Error("could not decode paid invoice", "op", op, "error", err)
			http.Error(w, "invalid webhook object", http.StatusBadRequest)
			return
		}
		payment := paymentInfoFromMetadata(invoice.Metadata)
		if err := h.sheets.AppendRecord(r.Context(), payment); err != nil {
			h.logger.Error("could not append paid invoice to Google Sheet", "op", op, "error", err)
			http.Error(w, "could not record payment", http.StatusInternalServerError)
			return
		}
		h.logger.Info("Stripe payment completed", "op", op, "event_id", event.ID, "event_type", event.Type)
	case stripe.EventTypeCheckoutSessionExpired, stripe.EventTypeInvoicePaymentFailed:
		h.logger.Info("Stripe payment failed", "op", op, "event_id", event.ID, "event_type", event.Type)
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
