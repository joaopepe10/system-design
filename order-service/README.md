# order-service

Porta: **8080**  
Tecnologia: Java 17 / Spring Boot 3 / Spring Data JPA / H2

O `order-service` é o **orquestrador central** do fluxo de compra. Ele recebe pedidos do cliente, consulta e reserva estoque no `inventory-service`, dispara a geração de nota fiscal no `invoice-service` e envia a notificação ao cliente pelo `notification-service` — tudo de forma **síncrona e bloqueante**.

---

## Entidades

### Order

| Campo         | Tipo          | Descrição                          |
|---------------|---------------|------------------------------------|
| id            | Long (PK)     | Identificador do pedido            |
| customerId    | Long          | ID do cliente                      |
| customerEmail | String        | E-mail do cliente                  |
| status        | OrderStatus   | Estado do pedido (`CONFIRMED`)     |
| totalAmount   | BigDecimal    | Valor total calculado              |
| createdAt     | LocalDateTime | Data/hora de criação               |
| items         | List<OrderItem> | Itens do pedido (OneToMany LAZY) |

### OrderItem

| Campo       | Tipo       | Descrição                        |
|-------------|------------|----------------------------------|
| id          | Long (PK)  | Identificador do item            |
| productId   | Long       | ID do produto no inventory       |
| productName | String     | Nome snapshot do produto         |
| quantity    | Integer    | Quantidade pedida                |
| unitPrice   | BigDecimal | Preço unitário no momento da compra |

### OrderStatus

```
CONFIRMED
```

---

## Endpoints

| Método | Path         | Descrição                  |
|--------|-------------|----------------------------|
| POST   | /orders      | Cria um novo pedido        |
| GET    | /orders      | Lista todos os pedidos     |
| GET    | /orders/{id} | Busca um pedido por ID     |

### POST /orders — Request Body

```json
{
  "customerId": 1,
  "customerEmail": "joao@exemplo.com",
  "items": [
    { "productId": 1, "quantity": 2 },
    { "productId": 3, "quantity": 1 }
  ]
}
```

### POST /orders — Response

```json
{
  "id": 1,
  "customerId": 1,
  "customerEmail": "joao@exemplo.com",
  "status": "CONFIRMED",
  "totalAmount": 7350.00,
  "createdAt": "2026-04-09T14:22:10",
  "items": [...]
}
```

---

## Fluxo Interno de createOrder

```
POST /orders
  │
  ├── Para cada item no pedido:
  │     ├── GET {inventoryUrl}/products/{productId}     (busca preço e nome)
  │     └── PUT {inventoryUrl}/products/{productId}/reserve  (desconta estoque)
  │
  ├── orderRepository.save(order)                       (persiste o pedido)
  │
  ├── POST {invoiceUrl}/invoices                        (gera NF — bloqueia 2s)
  │
  └── POST {notificationUrl}/notifications              (envia email — bloqueia 1.5s)
```

A resposta ao cliente só é enviada **depois** de todas essas etapas. Com 2 produtos no pedido, o tempo mínimo é ~3.5s.

---

## Configuração

```properties
server.port=8080
spring.datasource.hikari.maximum-pool-size=5   # BAD: muito baixo
spring.datasource.hikari.minimum-idle=2
```

As URLs dos serviços dependentes são lidas de variáveis de ambiente:

```
INVENTORY_SERVICE_URL   → inventory.service.url
INVOICE_SERVICE_URL     → invoice.service.url
NOTIFICATION_SERVICE_URL → notification.service.url
```

---

## Anti-padrões e Problemas

### Chamadas síncronas em sequência
Cada item do pedido gera 2 chamadas HTTP (`GET` + `PUT`). Depois, mais 2 chamadas (`invoice` + `notification`). Tudo bloqueante — a thread fica presa enquanto espera cada resposta.

### Sem transação distribuída
Se `invoice-service` falhar após as reservas de estoque terem sido feitas, o estoque fica reservado mas o pedido não existe para o cliente. Não há rollback.

### Sem timeout e sem retry
`RestTemplate` sem timeout configurado. Se qualquer serviço downstream travar, a thread do Tomcat fica presa indefinidamente.

### N+1 queries no GET /orders
`items` é `FetchType.LAZY`. Ao retornar a lista de pedidos, cada pedido dispara uma query adicional para buscar seus itens. Sem paginação, retorna todos os registros de uma vez.

```java
// BAD: LAZY loading -> N+1 queries ao listar pedidos
@OneToMany(cascade = CascadeType.ALL, fetch = FetchType.LAZY)
```

### Thread starvation
Com `invoice` (2s) + `notification` (1.5s) bloqueantes, cada request prende uma thread por ~3.5s. O pool padrão do Tomcat tem 200 threads — saturação em ~57 pedidos simultâneos.
