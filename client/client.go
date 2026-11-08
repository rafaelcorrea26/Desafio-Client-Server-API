package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	serverURL = "http://localhost:8080/cotacao"

	requestTimeout = 300 * time.Millisecond

	arquivoSaida = "cotacao.txt"
)

type cotacao struct {
	Bid string `json:"bid"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	bid, err := buscarCotacao(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.Fatal("erro ao buscar a cotação: timeout de 300ms excedido ao aguardar a resposta do servidor")
		}
		log.Fatalf("erro ao buscar a cotação: %v", err)
	}

	if err := salvarArquivo(arquivoSaida, bid); err != nil {
		log.Fatalf("erro ao salvar %s: %v", arquivoSaida, err)
	}

	log.Printf("cotação salva em %s -> Dólar: %s", arquivoSaida, bid)
}

func buscarCotacao(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("o servidor respondeu com status %s", resp.Status)
	}

	var c cotacao
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return "", err
	}

	if c.Bid == "" {
		return "", errors.New("resposta do servidor não trouxe o campo bid")
	}

	return c.Bid, nil
}

func salvarArquivo(caminho, bid string) error {
	return os.WriteFile(caminho, []byte("Dólar: "+bid), 0o644)
}
