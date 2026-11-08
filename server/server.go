package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	// driver do sqlite, o import vazio serve pra ele se registrar no database/sql
	_ "modernc.org/sqlite"
)

const (
	apiURL = "https://economia.awesomeapi.com.br/json/last/USD-BRL"

	apiTimeout = 200 * time.Millisecond
	dbTimeout  = 10 * time.Millisecond

	dbFile = "cotacoes.db"
	port   = ":8080"
)

type Cotacao struct {
	Code       string `json:"code"`
	Codein     string `json:"codein"`
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

// a api devolve tudo dentro de uma chave "USDBRL", por isso esse wrapper
type cotacaoResponse struct {
	USDBRL Cotacao `json:"USDBRL"`
}

type bidResponse struct {
	Bid string `json:"bid"`
}

func main() {
	db, err := abrirBanco(dbFile)
	if err != nil {
		log.Fatalf("erro ao abrir o banco de dados: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/cotacao", cotacaoHandler(db))

	log.Printf("servidor ouvindo em http://localhost%s/cotacao", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("erro ao subir o servidor: %v", err)
	}
}

func abrirBanco(caminho string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		return nil, err
	}

	// o sqlite não aceita duas gravações juntas, sem isso dá "database is locked"
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cotacoes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		code        TEXT,
		codein      TEXT,
		name        TEXT,
		high        TEXT,
		low         TEXT,
		var_bid     TEXT,
		pct_change  TEXT,
		bid         TEXT,
		ask         TEXT,
		timestamp   TEXT,
		create_date TEXT,
		criado_em   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// recebe o db pra não precisar deixar ele como variável global
func cotacaoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cotacao, err := buscarCotacao(ctx)
		if err != nil {
			log.Printf("erro ao buscar a cotação na API externa: %v", err)
			http.Error(w, "não foi possível obter a cotação", http.StatusInternalServerError)
			return
		}

		// se não conseguir gravar eu só logo e sigo, a cotação já foi buscada
		// e não faz sentido segurar ela do client por causa disso
		if err := salvarCotacao(ctx, db, cotacao); err != nil {
			log.Printf("erro ao persistir a cotação no banco: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(bidResponse{Bid: cotacao.Bid}); err != nil {
			log.Printf("erro ao escrever a resposta: %v", err)
		}
	}
}

func buscarCotacao(ctx context.Context) (Cotacao, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Cotacao{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Cotacao{}, erroAPI(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Cotacao{}, errors.New("a API de câmbio respondeu com status " + resp.Status)
	}

	var dados cotacaoResponse
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		return Cotacao{}, erroAPI(ctx, err)
	}

	if dados.USDBRL.Bid == "" {
		return Cotacao{}, errors.New("a API de câmbio não trouxe o campo bid")
	}

	return dados.USDBRL, nil
}

func salvarCotacao(ctx context.Context, db *sql.DB, c Cotacao) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	stmt, err := db.PrepareContext(ctx,
		`INSERT INTO cotacoes
			(code, codein, name, high, low, var_bid, pct_change, bid, ask, timestamp, create_date)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return erroBanco(ctx, err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx,
		c.Code, c.Codein, c.Name, c.High, c.Low, c.VarBid, c.PctChange, c.Bid, c.Ask, c.Timestamp, c.CreateDate,
	)
	if err != nil {
		return erroBanco(ctx, err)
	}

	return nil
}

func erroAPI(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("timeout de 200ms excedido ao chamar a API de câmbio")
	}
	return err
}

func erroBanco(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("timeout de 10ms excedido ao gravar no banco de dados")
	}
	return err
}
