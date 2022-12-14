package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	requestTimeout   time.Duration
	databaseTimeout  time.Duration
	serverPortNumber uint16
	db               *sql.DB
)

const (
	requestTimeoutUsage  string = "request timout usage: -rt 200ms or -rt 1s or -rt 1m"
	databaseTimeoutUsage string = "database timetout usage: -dbt 10ms or -dbt 1s"
	serverPortUsage      string = "server port usage: -p 8080 or -p 3000 (range from 0 to 65535)"
)

func main() {
	parseFlagValues()
	startDatabase()
	startHTTPServer()
}

func parseFlagValues() {
	var (
		reqTimeout string
		dbTimeout  string
		portNumber string
	)

	flag.StringVar(&reqTimeout, "rt", "200ms", requestTimeoutUsage)
	flag.StringVar(&dbTimeout, "dbt", "10ms", databaseTimeoutUsage)
	flag.StringVar(&portNumber, "p", "8080", serverPortUsage)
	flag.Parse()
	d, err := time.ParseDuration(reqTimeout)
	if err != nil {
		log.Fatalln("Invalid argument,", requestTimeoutUsage)
	}
	requestTimeout = d

	d, err = time.ParseDuration(dbTimeout)
	if err != nil {
		log.Fatalln("Invalid argument,", databaseTimeoutUsage)
	}
	databaseTimeout = d

	spn, err := strconv.ParseUint(portNumber, 10, 16)
	if err != nil {
		log.Fatalln("Invalid argument,", serverPortUsage)
	}
	serverPortNumber = uint16(spn)
}

func startDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "file:cotacao.db")
	if err != nil {
		log.Fatalln("Falhou abrir o banco de dados:", err)
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS cotacao(
		code TEXT, 
		code_in TEXT, 
		name TEXT, 
		high TEXT, 
		low TEXT,
		var_bid TEXT,
		pct_change TEXT,
		bid TEXT,
		ask TEXT,
		timestamp TEXT,
		create_date TEXT
	)`)
	if err != nil {
		log.Fatalln("Falha ao criar tabela de cotacao:", err)
	}
}

func startHTTPServer() {
	portNumber := fmt.Sprint(":", serverPortNumber)
	http.HandleFunc("/cotacao", cotacaoHandler)
	log.Println("Iniciando servidor na porta", portNumber)
	log.Println("Request timeout:", requestTimeout)
	log.Println("Database timeout:", databaseTimeout)
	err := http.ListenAndServe(portNumber, nil)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalln("*** ERROR ***:", err)
	}
}

func cotacaoHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("GET /cotacao")
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	cotacaoReq, err := http.NewRequestWithContext(ctx, "GET", cotacaoURL, nil)
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao criar requisi????o: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(cotacaoReq)
	if err != nil {
		var msg string
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprint("requisi????o ultrapassou o tempo m??ximo de ", requestTimeout)
		} else {
			msg = fmt.Sprint("GET /cotacao - requisi????o falhou: ", err)
		}
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var cotacao USDBRLQuotation
	err = json.NewDecoder(resp.Body).Decode(&cotacao)
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao decodificar corpo da requisi????o: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}

	err = saveQuotationToDB(r.Context(), &cotacao)
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao salvar dados no banco: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(QuotationResponse{cotacao.Bid})
	if err != nil {
		msg := fmt.Sprint("GET /cotacao - falha ao enviar requisi????o: ", err)
		sendMsgError(w, msg, http.StatusInternalServerError)
		return
	}
}

const cotacaoURL string = "https://economia.awesomeapi.com.br/json/last/USD-BRL"

func sendMsgError(w http.ResponseWriter, msg string, statusCode int) {
	log.Println(msg)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg, StatusCode: statusCode})
}

func saveQuotationToDB(ctx context.Context, cotacao *USDBRLQuotation) error {
	stmt, err := db.Prepare(`
		INSERT INTO cotacao(
			code,
			code_in,
			name,
			high,
			low,
			var_bid,
			pct_change,
			bid,
			ask,
			timestamp,
			create_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("falha ao preparar query. %w", err)
	}

	dbCtx, cancel := context.WithTimeout(ctx, databaseTimeout)
	defer cancel()

	_, err = stmt.ExecContext(
		dbCtx,
		cotacao.Code,
		cotacao.CodeIn,
		cotacao.Name,
		cotacao.High,
		cotacao.Low,
		cotacao.VarBid,
		cotacao.PctChange,
		cotacao.Bid,
		cotacao.Ask,
		cotacao.Timestamp,
		cotacao.CreateDate,
	)
	if err != nil {
		return fmt.Errorf("falha ao executar query. %w", err)
	}
	return nil
}

type ErrorResponse struct {
	Error      string `json:"error"`
	StatusCode int    `json:"status_code"`
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
