# notification-service

Porta: **8083**  
Tecnologia: Java 17 / Spring Boot 3 / Spring Data JPA / H2

O `notification-service` é responsável por enviar notificações ao cliente após a confirmação de um pedido (simula envio de e-mail). Ele introduz um atraso artificial de `Thread.sleep(1500)` — **1.5 segundos bloqueantes por requisição** — para simular a latência de um provedor de e-mail externo (SendGrid, SES, etc.).

É chamado pelo `order-service` de forma síncrona, **após** o `invoice-service`, como última etapa do fluxo de criação de pedido.

---

## Entidades

### Notification

| Campo         | Tipo               | Descrição                                    |
|---------------|--------------------|----------------------------------------------|
| id            | Long (PK)          | Identificador da notificação                 |
| orderId       | Long               | ID do pedido relacionado                     |
| customerEmail | String             | E-mail do destinatário                       |
| message       | String (max 1000)  | Conteúdo da mensagem enviada                 |
| status        | NotificationStatus | Status do envio (`SENT`)                     |
| sentAt        | LocalDateTime      | Data/hora do envio                           |

### NotificationStatus

```
SENT
```

---

## Endpoints

| Método | Path            | Descrição                         |
|--------|----------------|-----------------------------------|
| POST   | /notifications  | Envia uma notificação             |
| GET    | /notifications  | Lista todas as notificações       |

### POST /notifications — Request Body

```json
{
  "orderId": 1,
  "customerEmail": "joao@exemplo.com",
  "message": "Pedido #1 confirmado! Total: R$ 3850.00"
}
```

### POST /notifications — Response

Demora **~1.5 segundos** antes de responder.

```json
{
  "id": 1,
  "orderId": 1,
  "customerEmail": "joao@exemplo.com",
  "message": "Pedido #1 confirmado! Total: R$ 3850.00",
  "status": "SENT",
  "sentAt": "2026-04-09T14:22:14"
}
```

---

## Anti-padrões e Problemas

### Thread bloqueante por 1.5 segundos

```java
// BAD: Thread.sleep bloqueante simulando envio de email.
// 1.5s por requisição consumindo thread do Tomcat.
// Combinado com o invoice-service (2s), cada pedido bloqueia uma thread por ~3.5s+.
public Notification send(NotificationRequest request) {
    Thread.sleep(1500);
    // ...
}
```

### Posição no fluxo amplifica o problema

O `notification-service` é chamado **depois** do `invoice-service` (já 2s bloqueantes). A soma dos dois resulta em ~3.5s de bloqueio por pedido. Quanto mais pedidos simultâneos, mais rápido o pool de threads do Tomcat se esgota.

```
order-service chama:
  invoice (2.0s bloqueado)
  → notification (1.5s bloqueado)
  → total: ~3.5s com thread presa
```

### Solução ideal

Assim como o `invoice-service`, este deveria ser consumidor de um evento assíncrono (`OrderConfirmed`) publicado pelo `order-service`. O envio de e-mail não precisa bloquear a confirmação do pedido para o cliente — pode acontecer em background segundos depois.
