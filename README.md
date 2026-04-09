# Ecommerce — Laboratório de Anti-padrões em Microsserviços

Este projeto é um **laboratório didático** com uma arquitetura de microsserviços **intencionalmente problemática**. O objetivo é ter um ambiente real para observar, medir e depois corrigir problemas clássicos de sistemas distribuídos.

---

## Visão Geral da Arquitetura

```
Cliente HTTP
     │
     ▼
┌─────────────────┐   GET /products/{id}    ┌──────────────────────┐
│  order-service  │ ───────────────────────▶│  inventory-service   │
│    :8080        │ ◀─── Product ───────────│      :8081           │
│                 │                          └──────────────────────┘
│   Orquestrador  │   PUT /products/{id}/reserve
│   síncrono      │ ────────────────────────────────────────────────▶ (inventory)
│                 │
│                 │   POST /invoices         ┌──────────────────────┐
│                 │ ───────────────────────▶│  invoice-service     │
│                 │ ◀─── Invoice (2s) ──────│      :8082           │
│                 │                          └──────────────────────┘
│                 │
│                 │   POST /notifications    ┌──────────────────────┐
│                 │ ───────────────────────▶│ notification-service │
│                 │ ◀─── Notification(1.5s)─│      :8083           │
└─────────────────┘                          └──────────────────────┘

┌─────────────────┐
│   load-tester   │  ──▶  dispara cenários de teste contra todos os serviços
│    :9090        │
└─────────────────┘
```

### Fluxo de um pedido

```
POST /orders
  │
  ├─▶ [inventory] GET /products/{id}    — busca dados de cada produto (N chamadas)
  ├─▶ [inventory] PUT /reserve          — reserva estoque de cada produto (N chamadas)
  ├─▶ [invoice]   POST /invoices        — gera nota fiscal (bloqueia ~2s)
  └─▶ [notif]     POST /notifications   — envia e-mail (bloqueia ~1.5s)
        │
        ▼
    resposta ao cliente (~3.5s+)
```

Toda a comunicação é **HTTP síncrona e bloqueante** — cada serviço espera a resposta do próximo antes de liberar a thread.

---

## Serviços

| Serviço              | Porta | Tecnologia       | Banco        |
|----------------------|-------|------------------|--------------|
| order-service        | 8080  | Java / Spring Boot | H2 in-memory |
| inventory-service    | 8081  | Java / Spring Boot | H2 in-memory |
| invoice-service      | 8082  | Java / Spring Boot | H2 in-memory |
| notification-service | 8083  | Java / Spring Boot | H2 in-memory |
| load-tester          | 9090  | Go               | —            |

---

## Anti-padrões Implementados (Intencionais)

### 1. Comunicação síncrona em cadeia
`order-service` faz chamadas HTTP sequenciais para 3 serviços. Se qualquer um falhar ou demorar, o pedido inteiro trava. Latência total = soma de todas as latências.

### 2. Sem transação distribuída
Se a reserva de estoque do produto 2 falhar, o produto 1 já foi reservado e não é revertido. O sistema fica inconsistente sem nenhum mecanismo de compensação.

### 3. Race condition no estoque
`inventory-service` não usa `@Transactional` nem locking. Dois pedidos simultâneos podem ler o mesmo estoque, ambos passam na verificação e ambos subtraem — o estoque vai para negativo.

### 4. Thread starvation
`invoice-service` faz `Thread.sleep(2000)` e `notification-service` faz `Thread.sleep(1500)` simulando operações lentas. Como `order-service` chama os dois em sequência, cada pedido bloqueia **1 thread do Tomcat por ~3.5s**. Com o pool padrão de 200 threads, a saturação começa com ~57 pedidos simultâneos.

### 5. Pool de conexões subdimensionado
Todos os serviços têm `hikari.maximum-pool-size=5`. Sob carga, requisições ficam esperando conexão disponível no pool.

### 6. N+1 queries
`Order` carrega `items` com `FetchType.LAZY`. O endpoint `GET /orders` retorna todos os pedidos sem paginação, causando 1 query para a lista + 1 query por pedido para buscar os itens.

### 7. Sem timeout, sem retry, sem circuit breaker
Se qualquer serviço downstream demorar indefinidamente, a thread fica presa para sempre.

---

## Como Subir o Projeto

**Pré-requisitos:** Docker e Docker Compose instalados.

```bash
# Na raiz do projeto
docker-compose up --build
```

Aguarde todos os serviços subirem (pode levar 1-2 minutos no primeiro build).

### Verificar se tudo está de pé

```bash
curl http://localhost:8080/orders
curl http://localhost:8081/products
curl http://localhost:8082/invoices
curl http://localhost:8083/notifications
```

Ou acesse o load-tester em **http://localhost:9090** e clique em "Health Check".

---

## Criando um Pedido Manualmente

Os produtos são carregados automaticamente pelo inventory-service. Para saber os IDs:

```bash
curl http://localhost:8081/products
```

Exemplo de pedido:

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customerId": 1,
    "customerEmail": "joao@exemplo.com",
    "items": [
      { "productId": 1, "quantity": 1 },
      { "productId": 2, "quantity": 2 }
    ]
  }'
```

A resposta deve demorar **~3.5 segundos** — tempo que o pedido fica bloqueado esperando invoice + notification.

---

## Load Tester

O load-tester expõe uma interface web em **http://localhost:9090** com os seguintes cenários:

| Cenário          | O que demonstra                                      |
|------------------|------------------------------------------------------|
| Health Check     | Verifica se todos os serviços estão respondendo      |
| Latência         | Mede latência de cada serviço isoladamente           |
| Pedido Único     | Latência real da cadeia completa (~3.5s por pedido)  |
| Race Condition   | Estoque indo a negativo com pedidos concorrentes     |
| Thread Starvation| Esgotamento de threads sob carga simultânea          |
| Pool Exhaustion  | Esgotamento do pool de conexões HikariCP             |
| All              | Executa todos os cenários em sequência               |

Logs em tempo real:
```bash
docker-compose logs -f load-tester
```

---

## Estrutura de Diretórios

```
ecommerce/
├── docker-compose.yml
├── order-service/          # Orquestrador — cria pedidos
├── inventory-service/      # Gerencia produtos e estoque
├── invoice-service/        # Gera notas fiscais (lento: 2s)
├── notification-service/   # Envia notificações por e-mail (lento: 1.5s)
└── load-tester/            # Cenários de teste em Go
```

Cada serviço tem seu próprio `README.md` com detalhes das entidades, endpoints e anti-padrões específicos.

---

## Próximos Passos (O que Corrigir)

- [ ] Substituir chamadas síncronas para invoice e notification por **mensageria assíncrona** (Kafka / RabbitMQ)
- [ ] Adicionar **transação distribuída / Saga pattern** para reverter reservas em caso de falha
- [ ] Adicionar `@Transactional` com **pessimistic locking** no reserve de estoque
- [ ] Configurar **timeouts e circuit breaker** (Resilience4j) no order-service
- [ ] Redimensionar o pool de conexões e habilitar **paginação** nos endpoints de listagem
- [ ] Adicionar **index** no campo `name` da tabela de produtos
