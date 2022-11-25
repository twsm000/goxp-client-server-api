package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var (
	requestTimeout  time.Duration
	databaseTimeout time.Duration
)

const (
	requestTimeoutUsage  string = "request timout usage: -rt 200ms or -rt 1s or -rt 1m"
	databaseTimeoutUsage string = "database timetout usage: -dbt 10ms or -dbt 1s"
)

func main() {
	parseContextTimeouts()
	startHTTPServer()
}

func parseContextTimeouts() {
	var (
		reqTimeout string
		dbTimeout  string
	)

	flag.StringVar(&reqTimeout, "rt", "200ms", requestTimeoutUsage)
	flag.StringVar(&dbTimeout, "dbt", "10ms", databaseTimeoutUsage)
	flag.Parse()
	d, err := time.ParseDuration(reqTimeout)
	if err != nil {
		log.Fatalln("Invalid", requestTimeoutUsage)
	}
	requestTimeout = d

	d, err = time.ParseDuration(dbTimeout)
	if err != nil {
		log.Fatalln("Invalid", databaseTimeoutUsage)
	}
	databaseTimeout = d
}

func startHTTPServer() {
	http.HandleFunc("/cotacao", cotacaoHandler)
	log.Println("Iniciando servidor na porta :8080")
	log.Println("Request timeout:", requestTimeout)
	log.Println("Database timeout:", databaseTimeout)
	if err := http.ListenAndServe(":8080", nil); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalln("*** ERROR ***:", err)
	}
}

func cotacaoHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("GET /cotacao")
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	cotacaoReq, err := http.NewRequestWithContext(ctx, "GET", cotacaoURL, nil)
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao criar requisição: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(cotacaoReq)
	if err != nil {
		var msg string
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprint("requisição ultrapassou o tempo máximo de ", requestTimeout)
		} else {
			msg = fmt.Sprint("GET /cotacao - requisição falhou: ", err)
		}
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var cotacao USDBRLQuotation
	err = json.NewDecoder(resp.Body).Decode(&cotacao)
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao decodificar corpo da requisição: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(QuotationResponse{cotacao.Bid})
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao enviar requisição: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}
}

const cotacaoURL string = "https://economia.awesomeapi.com.br/json/last/USD-BRL"

func sendMsgError(w http.ResponseWriter, msg string, statusCode int) {
	log.Println(msg)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type QuotationResponse struct {
	Bid string `json:"bid"`
}

type USDBRLQuotation struct {
	Quotation `json:"USDBRL"`
}

type Quotation struct {
	Code       string `json:"code"`
	CodeIn     string `json:"codein"`
	Name       string `json:"name"`
	High       string `json:"high"`
	Low        string `json:"low"`
	VarBid     string `json:"varBid"`
	PctChange  string `json:"pctChange"`
	Bid        string `json:"bid"`
	Ask        string `json:"ask"`
	Timestamp  string `json:"timestamp"`
	CreateDate string `json:"create_date"`
}
