# load-tester

Porta: **9090**  
Tecnologia: Go (sem dependências externas)

O `load-tester` é uma ferramenta de testes de carga e demonstração de anti-padrões. Ele expõe uma **interface web** em `http://localhost:9090` onde você pode disparar cenários contra os outros serviços e observar os problemas em tempo real nos logs.

---

## Como Usar

### Via interface web (recomendado)

Acesse **http://localhost:9090** no browser e clique nos botões de cada cenário. Os resultados aparecem nos logs do container:

```bash
docker-compose logs -f load-tester
```

### Via linha de comando (sem Docker)

```bash
cd load-tester

# Verifica se todos os serviços estão no ar
go run main.go -scenario=health

# Mede latência de cada serviço individualmente
go run main.go -scenario=latency

# Cria um único pedido e mostra o tempo real
go run main.go -scenario=single

# Demonstra a race condition no estoque
go run main.go -scenario=race -concurrency=30

# Demonstra thread starvation
go run main.go -scenario=pool -concurrency=20

# Pedidos simultâneos — esgotamento de threads
go run main.go -scenario=concurrent -concurrency=20

# Executa todos os cenários em sequência
go run main.go -scenario=all -concurrency=20
```

---

## Cenários

### Health Check (`health`)
Faz um `GET` em cada serviço e verifica se estão respondendo. Ponto de partida antes de qualquer outro teste.

```
✓ order-service     :8080   HTTP 200   ▓
✓ inventory-service :8081   HTTP 200   ▓
✓ invoice-service   :8082   HTTP 200   ▓
✓ notification-svc  :8083   HTTP 200   ▓
```

---

### Latência Individual (`latency`)
Mede o tempo de resposta de cada serviço isoladamente, incluindo as operações lentas:

- `GET /products`, `GET /invoices`, `GET /notifications` → milissegundos
- `POST /invoices` → **~2 segundos** (Thread.sleep)
- `POST /notifications` → **~1.5 segundos** (Thread.sleep)

Revela onde estão os gargalos antes de testar a cadeia completa.

---

### Pedido Único (`single`)
Cria um único pedido passando pela cadeia completa e mostra o tempo total de parede:

```
✓ [POST /orders]   3.512s   HTTP 200   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
⚠ Uma thread do Tomcat ficou BLOQUEADA por 3.5s esperando invoice + notification
```

Demonstra que mesmo um único pedido demora ~3.5s por causa das chamadas síncronas.

---

### Race Condition (`race`)
Dispara N pedidos simultâneos pedindo o **mesmo produto** para demonstrar o problema de concorrência no `inventory-service`.

Como o `reserve` não usa `@Transactional` nem locking, múltiplas threads podem:
1. Ler o mesmo valor de estoque
2. Todas passarem na verificação de suficiência
3. Todas decrementarem — o estoque vai abaixo de zero

```bash
go run main.go -scenario=race -concurrency=30
```

Verifique o estoque depois:
```bash
curl http://localhost:8081/products/1
# stockQuantity pode ser negativo!
```

---

### Thread Starvation / Pool Exhaustion (`pool` e `concurrent`)
Dispara N pedidos simultâneos para mostrar o esgotamento de threads e conexões:

- Cada pedido bloqueia 1 thread por ~3.5s
- Com 20 requests simultâneos: 20 threads presas × 3.5s
- Novos requests que chegam enquanto as threads estão bloqueadas ficam na fila ou falham com timeout
- O pool HikariCP de 5 conexões esgota ainda mais rápido

```
✗ [req#18]  CONN ERROR  connection refused / timeout
```

---

## Saída do Terminal

O load-tester usa cores ANSI para facilitar a leitura:

| Símbolo | Significado                 |
|---------|-----------------------------|
| `✓`     | Requisição bem-sucedida     |
| `✗`     | Falha (erro ou HTTP 4xx/5xx)|
| `⚠`     | Aviso / ponto de atenção    |
| `▓▓▓`   | Barra de latência (verde < 1s, amarelo < 3s, vermelho ≥ 3s) |

O resumo ao final de cada cenário mostra: total, taxa de sucesso, min/max/avg/p50/p95/p99.

---

## Variáveis de Ambiente

| Variável                 | Padrão                    |
|--------------------------|---------------------------|
| ORDER_SERVICE_URL        | http://localhost:8080      |
| INVENTORY_SERVICE_URL    | http://localhost:8081      |
| INVOICE_SERVICE_URL      | http://localhost:8082      |
| NOTIFICATION_SERVICE_URL | http://localhost:8083      |

No Docker Compose, essas variáveis já estão configuradas para os nomes dos containers.
