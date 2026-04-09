# invoice-service

Porta: **8082**  
Tecnologia: Java 17 / Spring Boot 3 / Spring Data JPA / H2

O `invoice-service` é responsável por gerar notas fiscais para pedidos confirmados. Ele simula um processo lento (geração de PDF, integração com sistema fiscal, etc.) através de um `Thread.sleep(2000)` — **2 segundos bloqueantes por requisição**.

É chamado diretamente pelo `order-service` de forma síncrona durante o fluxo de criação de pedido.

---

## Entidades

### Invoice

| Campo         | Tipo          | Descrição                               |
|---------------|---------------|-----------------------------------------|
| id            | Long (PK)     | Identificador da nota fiscal            |
| orderId       | Long          | ID do pedido de origem                  |
| customerId    | Long          | ID do cliente                           |
| totalAmount   | BigDecimal    | Valor total da nota                     |
| invoiceNumber | String        | Número único da NF (ex: `NF-42-16...`) |
| status        | InvoiceStatus | Status da nota (`GENERATED`)            |
| createdAt     | LocalDateTime | Data/hora de emissão                    |

### InvoiceStatus

```
GENERATED
```

---

## Endpoints

| Método | Path          | Descrição                        |
|--------|--------------|----------------------------------|
| POST   | /invoices     | Gera uma nova nota fiscal        |
| GET    | /invoices     | Lista todas as notas fiscais     |
| GET    | /invoices/{id}| Busca nota fiscal por ID         |

### POST /invoices — Request Body

```json
{
  "orderId": 1,
  "customerId": 42,
  "totalAmount": 3850.00
}
```

### POST /invoices — Response

Demora **~2 segundos** antes de responder.

```json
{
  "id": 1,
  "orderId": 1,
  "customerId": 42,
  "totalAmount": 3850.00,
  "invoiceNumber": "NF-1-1712669000000",
  "status": "GENERATED",
  "createdAt": "2026-04-09T14:22:12"
}
```

---

## Anti-padrões e Problemas

### Thread bloqueante por 2 segundos

```java
// BAD: Thread.sleep bloqueante simulando geração de PDF.
// Isso consome uma thread do pool do Tomcat por 2 segundos por requisição.
// Sob carga, o pool de threads do servidor esgota rapidamente.
public Invoice generate(InvoiceRequest request) {
    Thread.sleep(2000);
    // ...
}
```

Cada chamada a `POST /invoices` prende uma thread do servidor por 2 segundos. Como o `order-service` chama esse endpoint de forma **síncrona**, essa latência se soma diretamente ao tempo de resposta do pedido.

### Impacto no sistema

Com o `notification-service` somando mais 1.5s, cada pedido criado pelo `order-service` bloqueia uma thread por **~3.5 segundos**. Com o pool padrão do Tomcat (200 threads), a saturação começa com aproximadamente **57 pedidos simultâneos**.

### Solução ideal

Este serviço deveria ser chamado de forma **assíncrona via mensageria** (ex: Kafka, RabbitMQ). O `order-service` publicaria um evento `OrderConfirmed` e o `invoice-service` consumiria esse evento em background, sem bloquear o fluxo principal.
