package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const sheetsOAuthScope = "https://www.googleapis.com/auth/spreadsheets"

type Sheets struct {
	service *sheets.Service
	logger  *slog.Logger

	spreadsheet string
	appendRange string
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func NewSheets(ctx context.Context, logger *slog.Logger, credentialsFile, spreadsheetID, appendRange string) *Sheets {
	op := "sheets.NewSheets"

	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		logger.Error("unable to read client secret file", "op", op, "error", err)
		os.Exit(1)
	}

	config, err := google.ConfigFromJSON(b, sheetsOAuthScope)
	if err != nil {
		logger.Error("unable to parse client secret file to config", "op", op, "error", err)
		os.Exit(1)
	}

	tok := getTokenFromWeb(config)
	client := config.Client(context.Background(), tok)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		logger.Error("unable to retrieve Sheets client", "op", op, "error", err)
		os.Exit(1)
	}

	return &Sheets{
		service:     srv,
		logger:      logger,
		spreadsheet: spreadsheetID,
		appendRange: appendRange,
	}
}

func (s *Sheets) AppendRecord(ctx context.Context, payment PaymentInfo) error {
	op := "sheets.AppendRecord"

	values := [][]any{{
		payment.FullName,
		payment.Email,
		payment.Phone,
		payment.StartDate,
		payment.CourseName,
	}}

	_, err := s.service.Spreadsheets.Values.Append(s.spreadsheet, s.appendRange, &sheets.ValueRange{
		Values: values,
	}).ValueInputOption("RAW").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	if err != nil {
		s.logger.Error("append payment", "op", op, "error", err)
		return err
	}
	return nil
}

type PaymentInfo struct {
	FullName   string
	Email      string
	Phone      string
	StartDate  string
	CourseName string
}

func paymentInfoFromMetadata(metadata map[string]string) PaymentInfo {
	return PaymentInfo{
		FullName:   metadata["full_name"],
		Email:      metadata["email"],
		Phone:      metadata["phone"],
		StartDate:  metadata["start_date"],
		CourseName: metadata["course_name"],
	}
}
