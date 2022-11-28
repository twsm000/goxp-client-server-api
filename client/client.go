package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	requestTimeout time.Duration
)

const (
	fileName            string = "cotacao.txt"
	requestTimeoutUsage string = "request timout usage: -rt 300ms or -rt 1s or -rt 1m"
)

func main() {
	parseFlagValues()
	makeRequest()
}

func parseFlagValues() {
	var (
		reqTimeout string
	)

	flag.StringVar(&reqTimeout, "rt", "200ms", requestTimeoutUsage)
	flag.Parse()
	d, err := time.ParseDuration(reqTimeout)
	if err != nil {
		log.Fatalln("Invalid argument,", requestTimeoutUsage)
	}
	requestTimeout = d
}

func makeRequest() {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/cotacao", nil)
	if err != nil {
		log.Fatalln("Falha ao criar requisição:", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var msg string
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprint("Requisição ultrapassou o tempo máximo de ", requestTimeout)
		} else {
			msg = fmt.Sprint("Requisição falhou: ", err)
		}
		log.Fatalln(msg)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		saveQuotationToFile(resp.Body)
	default:
		handleError(resp.Body)
	}
}

func saveQuotationToFile(r io.Reader) {
	var cotacao QuotationResponse
	err := json.NewDecoder(r).Decode(&cotacao)
	if err != nil {
		log.Fatalln("Falha ao decodificar corpo da requisição:", err)
	}

	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer file.Close()

	msg := fmt.Sprint("Dólar: ", cotacao.Bid)
	_, err = fmt.Fprintln(file, msg)
	if err != nil {
		log.Fatalln("Falha ao salvar dados em disco:", err)
	}
	log.Println("Registro salvo em disco.", msg)
}

func handleError(r io.Reader) {
	var errResp ErrorResponse
	err := json.NewDecoder(r).Decode(&errResp)
	if err != nil {
		log.Fatalln("Falha ao decodificar corpo da requisição:", err)
	}
	log.Fatalf("Ocorreu um erro: %s\nCódigo: %d\n", errResp.Error, errResp.StatusCode)
}

type ErrorResponse struct {
	Error      string `json:"error"`
	StatusCode int    `json:"status_code"`
}

type QuotationResponse struct {
	Bid string `json:"bid"`
}
