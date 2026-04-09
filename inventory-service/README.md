# inventory-service

Porta: **8081**  
Tecnologia: Java 17 / Spring Boot 3 / Spring Data JPA / H2

O `inventory-service` gerencia o catálogo de produtos e o controle de estoque. Ele é chamado pelo `order-service` para consultar dados de produtos e para reservar (decrementar) o estoque quando um pedido é criado.

Na inicialização, carrega automaticamente **5 produtos** de exemplo no banco.

---

## Entidades

### Product

| Campo         | Tipo       | Descrição                                      |
|---------------|------------|------------------------------------------------|
| id            | Long (PK)  | Identificador do produto                       |
| name          | String     | Nome do produto *(sem index — full table scan)*|
| description   | String     | Descrição detalhada                            |
| price         | BigDecimal | Preço unitário                                 |
| stockQuantity | Integer    | Quantidade disponível em estoque               |

---

## Produtos Pré-carregados

| ID | Nome                    | Preço      | Estoque |
|----|-------------------------|------------|---------|
| 1  | Notebook Dell Inspiron  | R$ 3.500   | 10      |
| 2  | Mouse Logitech MX Master| R$ 350     | 50      |
| 3  | Teclado Mecânico RGB    | R$ 550     | 30      |
| 4  | Monitor LG 27" 4K       | R$ 2.800   | 15      |
| 5  | Headset HyperX Cloud    | R$ 450     | 25      |

---

## Endpoints

| Método | Path                        | Descrição                          |
|--------|-----------------------------|------------------------------------|
| GET    | /products                   | Lista todos os produtos            |
| GET    | /products/{id}              | Busca produto por ID               |
| POST   | /products                   | Cria um novo produto               |
| PUT    | /products/{id}/reserve      | Reserva (decrementa) estoque       |

### PUT /products/{id}/reserve — Request Body

```json
{
  "quantity": 2
}
```

### PUT /products/{id}/reserve — Response

Retorna o produto com o estoque atualizado.

```json
{
  "id": 1,
  "name": "Notebook Dell Inspiron",
  "description": "...",
  "price": 3500.00,
  "stockQuantity": 8
}
```

Se o estoque for insuficiente, retorna `500` com a mensagem:
```
Estoque insuficiente para o produto 1. Disponivel: 10, Solicitado: 15
```

---

## Anti-padrões e Problemas

### Race condition clássica no reserve

Este é o problema mais crítico do serviço. O método `reserve` não usa `@Transactional` e não aplica nenhum tipo de locking:

```java
// Fluxo com race condition:
// Thread A: findById → stockQuantity = 10
// Thread B: findById → stockQuantity = 10  (lê o mesmo valor)
// Thread A: verifica 10 >= 5 → passa
// Thread B: verifica 10 >= 5 → passa
// Thread A: save(stockQuantity = 5)
// Thread B: save(stockQuantity = 5)  ← deveria ser 0, mas sobrescreve A
```

Com 30 pedidos simultâneos pedindo o mesmo produto, o estoque pode ir **abaixo de zero**.

**Solução correta:** usar `@Transactional` + `@Lock(LockModeType.PESSIMISTIC_WRITE)` no repository, ou atualização com query atômica:
```sql
UPDATE products SET stock_quantity = stock_quantity - :qty
WHERE id = :id AND stock_quantity >= :qty
```

### Sem índice no campo `name`

```java
// BAD: sem index no name - buscas por nome fazem full table scan
private String name;
```

Qualquer busca por nome varre a tabela inteira.

### Pool de conexões subdimensionado

```properties
spring.datasource.hikari.maximum-pool-size=5
```

Com 5 conexões simultâneas no máximo, sob carga concorrente as requisições ficam na fila esperando uma conexão livre.
