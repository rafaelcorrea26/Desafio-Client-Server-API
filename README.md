# Desafio Client-Server API

Desafio de Go usando http, context, banco de dados e manipulação de arquivos.

São dois programas separados. O client pede a cotação do dólar pro server, o
server busca essa cotação numa API externa, guarda no sqlite e devolve o
resultado. Cada etapa tem um tempo limite diferente:

- client esperando o server: 300ms
- server chamando a API de câmbio: 200ms
- server gravando no banco: 10ms

Tudo isso feito com o pacote context. Se algum desses prazos estoura, o erro
aparece no console de quem estourou.

## Como rodar

Precisa ter o Go 1.26.1 ou mais novo, que é a versão do `go.mod`.

Num terminal, sobe o server:

```
go run ./server
```

Ele vai imprimir `servidor ouvindo em http://localhost:8080/cotacao` e ficar
parado esperando requisição.

Em **outro** terminal, na mesma pasta, roda o client:

```
go run ./client
```

Se der tudo certo ele imprime a cotação e cria o arquivo `cotacao.txt` com o
valor dentro, no formato `Dólar: 5.1527`.

Pra testar só o endpoint, sem o client:

```
curl http://localhost:8080/cotacao
```

Ele devolve só o valor atual, tipo `{"bid":"5.1527"}`.

## Organização dos arquivos

```
client/client.go   faz a requisição e grava o cotacao.txt
server/server.go   endpoint /cotacao na porta 8080 e gravação no banco
```

Os dois estão em pastas separadas porque os dois são `package main` e têm uma
`func main()` — se ficassem soltos na mesma pasta o Go não compila.

O `cotacoes.db` e o `cotacao.txt` são criados na hora que os programas rodam,
por isso não estão no repositório.

## Banco de dados

O server cria o arquivo `cotacoes.db` e a tabela sozinho na primeira vez que
sobe, não precisa rodar nenhum script antes. Cada cotação buscada é gravada
como uma linha nova na tabela `cotacoes`. Se a gravação falhar, o erro vai pro
log do server.

Pra conferir o que foi gravado, se tiver o sqlite3 instalado:

```
sqlite3 cotacoes.db "select id, bid, criado_em from cotacoes"
```

Usei o driver `modernc.org/sqlite` e não o `mattn/go-sqlite3`, que é o mais
conhecido. O mattn depende de CGO e precisa de um compilador C instalado na
máquina pra compilar, e eu não tinha. O modernc é escrito em Go puro, então
compila em qualquer lugar sem instalar nada.

## Observações sobre os timeouts

O de 10ms do banco é curto. O sqlite grava em disco de verdade, então às vezes
ele passa desse tempo. Quando isso acontece o erro vai pro log do server, mas a
cotação ainda é devolvida pro client, porque ela já tinha sido buscada.

Esse log não quer dizer que a linha se perdeu. Nos testes com 10ms, as linhas
que deram timeout ficaram gravadas. Com prazos bem menores, parte delas não
ficou.

O server usa uma conexão só com o banco (`SetMaxOpenConns(1)`), porque o sqlite
não aceita duas gravações ao mesmo tempo. Com várias requisições juntas elas
fazem fila, e as que passam dos 10ms esperando podem não ser gravadas.

O contexto do server é derivado do `r.Context()`, que é o contexto da própria
requisição. Isso quer dizer que se o client desistir antes (nos 300ms dele), o
server para de esperar a API na hora em vez de continuar trabalhando à toa.
