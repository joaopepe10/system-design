package main

// ══════════════════════════════════════════════════════════════════════════════
// Ecommerce Load Tester
//
// Modo CLI:
//   go run main.go -scenario=health
//   go run main.go -scenario=single
//   go run main.go -scenario=latency
//   go run main.go -scenario=race       -concurrency=30
//   go run main.go -scenario=pool       -concurrency=20
//   go run main.go -scenario=concurrent -concurrency=20
//   go run main.go -scenario=all        -concurrency=20
//
// Modo servidor HTTP:
//   go run main.go -server -port=9090
//   Acesse http://localhost:9090 no browser
// ══════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────
// ANSI colors
// ─────────────────────────────────────────────

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
)

// ─────────────────────────────────────────────
// CONFIG
// ─────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	orderURL     = envOr("ORDER_SERVICE_URL", "http://localhost:8080")
	inventoryURL = envOr("INVENTORY_SERVICE_URL", "http://localhost:8081")
	invoiceURL   = envOr("INVOICE_SERVICE_URL", "http://localhost:8082")
	notifURL     = envOr("NOTIFICATION_SERVICE_URL", "http://localhost:8083")
)

// ─────────────────────────────────────────────
// HTTP CLIENT
// ─────────────────────────────────────────────

type Result struct {
	ID         int
	Label      string
	StatusCode int
	Duration   time.Duration
	Err        error
	Body       string
}

func (r Result) OK() bool {
	return r.Err == nil && r.StatusCode >= 200 && r.StatusCode < 300
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

func doRequest(method, url string, body interface{}) Result {
	start := time.Now()
	var br io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		br = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, br)
	if err != nil {
		return Result{Err: err, Duration: time.Since(start)}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	dur := time.Since(start)
	if err != nil {
		return Result{Err: err, Duration: dur}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return Result{StatusCode: resp.StatusCode, Duration: dur, Body: string(b)}
}

// ─────────────────────────────────────────────
// OUTPUT / PRINTER
// ─────────────────────────────────────────────

var outputMu sync.Mutex

func logLine(format string, args ...interface{}) {
	outputMu.Lock()
	fmt.Printf(format+"\n", args...)
	outputMu.Unlock()
}

func banner(title string) {
	line := strings.Repeat("═", 64)
	fmt.Printf("\n%s%s%s\n", cBold+cBlue, line, cReset)
	fmt.Printf("%s  %-60s%s\n", cBold+cCyan, title, cReset)
	fmt.Printf("%s%s%s\n\n", cBold+cBlue, line, cReset)
}

func section(title string) {
	fmt.Printf("\n  %s%s%s\n", cBold, title, cReset)
}

func info(format string, args ...interface{}) {
	fmt.Printf(cGray+"  → "+cReset+format+"\n", args...)
}

func hint(format string, args ...interface{}) {
	fmt.Printf(cYellow+"  ⚠  "+cReset+cYellow+format+cReset+"\n", args...)
}

func bad(format string, args ...interface{}) {
	fmt.Printf(cRed+"  ✗  "+format+cReset+"\n", args...)
}

func good(format string, args ...interface{}) {
	fmt.Printf(cGreen+"  ✓  "+cReset+format+"\n", args...)
}

func printResult(r Result) {
	icon := cGreen + "✓" + cReset
	scStr := fmt.Sprintf("HTTP %d", r.StatusCode)
	scColor := cGreen

	if !r.OK() {
		icon = cRed + "✗" + cReset
		scColor = cRed
	}

	durColor := cGreen
	if r.Duration > 3*time.Second {
		durColor = cRed
	} else if r.Duration > 1*time.Second {
		durColor = cYellow
	}

	label := r.Label
	if label == "" {
		label = fmt.Sprintf("req#%d", r.ID)
	}

	bar := buildBar(r.Duration)

	outputMu.Lock()
	if r.Err != nil {
		fmt.Printf("  %s [%-10s] %s%-10s%s  %sCONN ERROR%s  %s%s%s\n",
			icon, label,
			durColor, r.Duration.Round(time.Millisecond), cReset,
			cRed, cReset,
			cGray, r.Err.Error(), cReset,
		)
	} else {
		fmt.Printf("  %s [%-10s] %s%-10s%s  %s%-10s%s  %s\n",
			icon, label,
			durColor, r.Duration.Round(time.Millisecond), cReset,
			scColor, scStr, cReset,
			bar,
		)
	}
	outputMu.Unlock()
}

func buildBar(d time.Duration) string {
	ms := d.Milliseconds()
	blocks := int(ms / 200)
	if blocks > 28 {
		blocks = 28
	}
	if blocks < 1 {
		blocks = 1
	}
	color := cGreen
	if ms > 3000 {
		color = cRed
	} else if ms > 1000 {
		color = cYellow
	}
	return color + strings.Repeat("▓", blocks) + cReset
}

func printSummary(title string, results []Result) {
	total := len(results)
	if total == 0 {
		return
	}
	success := 0
	var durs []float64
	for _, r := range results {
		if r.OK() {
			success++
		}
		durs = append(durs, float64(r.Duration.Milliseconds()))
	}
	sort.Float64s(durs)

	minD := durs[0]
	maxD := durs[len(durs)-1]
	var sum float64
	for _, d := range durs {
		sum += d
	}
	avg := sum / float64(len(durs))
	p50 := pct(durs, 50)
	p95 := pct(durs, 95)
	p99 := pct(durs, 99)

	successRate := float64(success) / float64(total) * 100
	srColor := cGreen
	if successRate < 80 {
		srColor = cRed
	} else if successRate < 95 {
		srColor = cYellow
	}

	line := strings.Repeat("─", 64)
	fmt.Printf("\n  %s%s%s\n", cBold, line, cReset)
	if title != "" {
		fmt.Printf("  %s  RESUMO: %s%s\n", cBold, title, cReset)
	} else {
		fmt.Printf("  %s  RESUMO%s\n", cBold, cReset)
	}
	fmt.Printf("  %s%s%s\n", cBold, line, cReset)
	fmt.Printf("  %-22s %s%d / %d  (%.0f%%)%s\n",
		"Sucessos:", srColor, success, total, successRate, cReset)
	fmt.Printf("  %-22s %s%d%s\n",
		"Falhas:", cRed, total-success, cReset)
	fmt.Printf("  %-22s %s%.0f ms%s\n", "Mínimo:", cGreen, minD, cReset)
	fmt.Printf("  %-22s %s%.0f ms%s\n", "Média:", cYellow, avg, cReset)
	fmt.Printf("  %-22s %.0f ms\n", "P50 (mediana):", p50)
	fmt.Printf("  %-22s %s%.0f ms%s\n", "P95:", cYellow, p95, cReset)
	fmt.Printf("  %-22s %s%.0f ms%s\n", "P99:", cRed, p99, cReset)
	fmt.Printf("  %-22s %s%.0f ms%s\n", "Máximo:", cRed, maxD, cReset)
	fmt.Printf("  %s%s%s\n", cBold, line, cReset)
}

func pct(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*p/100)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func pause(d time.Duration) {
	fmt.Printf(cGray+"\n  aguardando %s antes do próximo cenário...\n"+cReset, d)
	time.Sleep(d)
}

// ─────────────────────────────────────────────
// REQUEST PAYLOADS
// ─────────────────────────────────────────────

func orderPayload(p1, p2 int64) map[string]interface{} {
	return map[string]interface{}{
		"customerId":    1,
		"customerEmail": "load@tester.com",
		"items": []map[string]interface{}{
			{"productId": p1, "quantity": 1},
			{"productId": p2, "quantity": 1},
		},
	}
}

func reservePayload(qty int) map[string]interface{} {
	return map[string]interface{}{"quantity": qty}
}

// fetchProducts busca os produtos do inventory e retorna ou para com erro
func fetchProducts() []map[string]interface{} {
	r := doRequest("GET", inventoryURL+"/products", nil)
	if !r.OK() {
		bad("Não foi possível listar produtos do inventory-service (status %d)", r.StatusCode)
		if r.Err != nil {
			bad("Erro de conexão: %s", r.Err)
		}
		return nil
	}
	var products []map[string]interface{}
	json.Unmarshal([]byte(r.Body), &products)
	if len(products) < 2 {
		bad("inventory-service retornou menos de 2 produtos.")
		return nil
	}
	return products
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 0: HEALTH CHECK
// ═══════════════════════════════════════════════════════════════

func scenarioHealthCheck() {
	banner("HEALTH CHECK — VERIFICANDO TODOS OS SERVIÇOS")

	services := []struct {
		label string
		url   string
	}{
		{"order-service     :8080", orderURL + "/orders"},
		{"inventory-service :8081", inventoryURL + "/products"},
		{"invoice-service   :8082", invoiceURL + "/invoices"},
		{"notification-svc  :8083", notifURL + "/notifications"},
	}

	allUp := true
	for _, s := range services {
		r := doRequest("GET", s.url, nil)
		r.Label = s.label
		printResult(r)
		if !r.OK() {
			allUp = false
		}
	}

	fmt.Println()
	if allUp {
		good("Todos os serviços estão respondendo!")
	} else {
		bad("Um ou mais serviços estão fora do ar.")
		info("Suba os serviços com: cd ecommerce && docker-compose up --build")
	}
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 1: LATÊNCIA POR SERVIÇO
// ═══════════════════════════════════════════════════════════════

func scenarioLatency() {
	banner("CENÁRIO 1: LATÊNCIA INDIVIDUAL POR SERVIÇO")

	info("Mede o tempo de resposta de cada serviço isoladamente.")
	info("Revela onde estão os gargalos antes de testar a cadeia completa.")

	// ── Serviços de leitura ──────────────────────────────────────
	reads := []struct {
		label string
		url   string
	}{
		{"inventory GET /products", inventoryURL + "/products"},
		{"invoice   GET /invoices", invoiceURL + "/invoices"},
		{"notif     GET /notifics", notifURL + "/notifications"},
	}

	for _, s := range reads {
		section(s.label)
		var results []Result
		for i := 1; i <= 5; i++ {
			r := doRequest("GET", s.url, nil)
			r.ID = i
			r.Label = fmt.Sprintf("%d/5", i)
			results = append(results, r)
			printResult(r)
		}
		printSummary("", results)
	}

	// ── Invoice POST (bloqueante 2s) ─────────────────────────────
	section("invoice POST /invoices  ← sleep de 2s!")
	invoiceBody := map[string]interface{}{
		"orderId": 99999, "customerId": 1, "totalAmount": 100.00,
	}
	var invResults []Result
	for i := 1; i <= 3; i++ {
		r := doRequest("POST", invoiceURL+"/invoices", invoiceBody)
		r.ID = i
		r.Label = fmt.Sprintf("%d/3", i)
		invResults = append(invResults, r)
		printResult(r)
	}
	printSummary("", invResults)

	// ── Notification POST (bloqueante 1.5s) ─────────────────────
	section("notification POST /notifications  ← sleep de 1.5s!")
	notifBody := map[string]interface{}{
		"orderId": 99999, "customerEmail": "test@test.com", "message": "teste",
	}
	var notifResults []Result
	for i := 1; i <= 3; i++ {
		r := doRequest("POST", notifURL+"/notifications", notifBody)
		r.ID = i
		r.Label = fmt.Sprintf("%d/3", i)
		notifResults = append(notifResults, r)
		printResult(r)
	}
	printSummary("", notifResults)

	fmt.Println()
	hint("invoice POST = ~2s  |  notification POST = ~1.5s")
	hint("order-service chama os DOIS em sequência → cada pedido leva ~3.5s+")
	hint("Solução futura: tornar invoice e notification assíncronos (mensageria)")
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 2: PEDIDO ÚNICO — LATÊNCIA DA CADEIA COMPLETA
// ═══════════════════════════════════════════════════════════════

func scenarioSingleOrder() {
	banner("CENÁRIO 2: PEDIDO ÚNICO — LATÊNCIA DA CADEIA SÍNCRONA")

	info("Cria um único pedido passando pela cadeia completa:")
	info("order → inventory (reserve) → invoice (2s) → notification (1.5s)")
	fmt.Println()

	products := fetchProducts()
	if products == nil {
		return
	}
	p1 := int64(products[0]["id"].(float64))
	p2 := int64(products[1]["id"].(float64))
	info("Produtos: #%d (%s)  e  #%d (%s)",
		p1, products[0]["name"], p2, products[1]["name"])

	fmt.Println()
	info("Enviando POST %s/orders ...", orderURL)
	fmt.Println()

	wallStart := time.Now()
	r := doRequest("POST", orderURL+"/orders", orderPayload(p1, p2))
	r.Label = "POST /orders"
	printResult(r)
	wall := time.Since(wallStart)

	fmt.Println()
	if r.OK() {
		var order map[string]interface{}
		json.Unmarshal([]byte(r.Body), &order)
		good("Pedido criado com sucesso!")
		if id, ok := order["id"]; ok {
			info("ID do pedido : %.0f", id)
		}
		if total, ok := order["totalAmount"]; ok {
			info("Total        : R$ %v", total)
		}
	} else {
		bad("Falha ao criar pedido:")
		fmt.Printf(cGray+"       %s\n"+cReset, r.Body)
	}

	fmt.Println()
	hint("Tempo de parede: %s", wall.Round(time.Millisecond))
	hint("Uma thread do Tomcat ficou BLOQUEADA por %.1fs esperando invoice + notification", wall.Seconds())
	hint("Com 200 threads padrão, saturação começa a partir de ~57 pedidos simultâneos")
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 3: PEDIDOS SIMULTÂNEOS — ESGOTAMENTO DE THREADS
// ═══════════════════════════════════════════════════════════════

func scenarioConcurrentOrders(n int) {
	banner(fmt.Sprintf("CENÁRIO 3: %d PEDIDOS SIMULTÂNEOS — THREAD STARVATION", n))

	info("Cada pedido bloqueia 1 thread do Tomcat por ~3.5s (invoice + notification)")
	info("Com %d requests simultâneos: %d threads presas × 3.5s = pressão no servidor", n, n)
	info("Requests novos que chegam enquanto as %d estão bloqueadas vão enfileirar ou falhar", n)
	fmt.Println()

	products := fetchProducts()
	if products == nil {
		return
	}
	p1 := int64(products[0]["id"].(float64))
	p2 := int64(products[1]["id"].(float64))

	info("Disparando %d goroutines simultaneamente para POST /orders ...", n)
	fmt.Println()

	var mu sync.Mutex
	var results []Result
	var wg sync.WaitGroup
	wallStart := time.Now()

	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r := doRequest("POST", orderURL+"/orders", orderPayload(p1, p2))
			r.ID = id
			r.Label = fmt.Sprintf("ord#%-3d", id)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			printResult(r)
		}(i)
	}

	wg.Wait()
	wall := time.Since(wallStart)

	fmt.Printf("\n  %sWall time total: %s%s%s\n",
		cBold, cCyan, wall.Round(time.Millisecond), cReset)

	printSummary(fmt.Sprintf("%d pedidos simultâneos", n), results)

	fmt.Println()
	hint("Wall time ≈ tempo do pedido mais lento (não N × 3.5s, porque rodam em paralelo)")
	hint("Mas com N grande, o próprio order-service começa a rejeitar ou atrasar requests")
	hint("Solução: tornar invoice/notification assíncronos para liberar threads rapidamente")
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 4: RACE CONDITION NO ESTOQUE
// ═══════════════════════════════════════════════════════════════

func scenarioRaceCondition(n int) {
	banner("CENÁRIO 4: RACE CONDITION — ESTOQUE INCONSISTENTE")

	stock := n / 2 // metade do número de requests = requests a mais que o estoque
	info("Estratégia: criar produto com %d unidades e disparar %d reservas simultâneas", stock, n)
	info("Correto:     exatamente %d aceitos, %d rejeitados, estoque final = 0", stock, n-stock)
	info("Com bug:     mais de %d aceitos → estoque fica NEGATIVO", stock)
	info("Causa raiz:  falta de @Transactional + locking em ProductService.reserve()")
	fmt.Println()

	// Cria produto específico para este teste
	newProduct := map[string]interface{}{
		"name":          fmt.Sprintf("Produto-RaceTest-%d", time.Now().UnixMilli()),
		"description":   "Produto criado pelo load-tester para testar race condition",
		"price":         49.90,
		"stockQuantity": stock,
	}
	info("Criando produto com stockQuantity=%d ...", stock)
	r := doRequest("POST", inventoryURL+"/products", newProduct)
	if !r.OK() {
		bad("Falha ao criar produto: %s", r.Body)
		return
	}
	var created map[string]interface{}
	json.Unmarshal([]byte(r.Body), &created)
	productID := int64(created["id"].(float64))
	good("Produto criado: ID=%d, estoque=%d", productID, stock)

	fmt.Println()
	info("Disparando %d reservas simultâneas (quantity=1) para o produto %d ...", n, productID)
	fmt.Println()

	var mu sync.Mutex
	var results []Result
	var wg sync.WaitGroup

	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			url := fmt.Sprintf("%s/products/%d/reserve", inventoryURL, productID)
			r := doRequest("PUT", url, reservePayload(1))
			r.ID = id
			r.Label = fmt.Sprintf("rsv#%-3d", id)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			printResult(r)
		}(i)
	}
	wg.Wait()

	// Verifica estoque final
	fmt.Println()
	info("Consultando estoque final do produto %d ...", productID)
	checkR := doRequest("GET", fmt.Sprintf("%s/products/%d", inventoryURL, productID), nil)

	successCount := 0
	for _, res := range results {
		if res.OK() {
			successCount++
		}
	}

	if checkR.OK() {
		var finalProduct map[string]interface{}
		json.Unmarshal([]byte(checkR.Body), &finalProduct)
		finalStock := int(finalProduct["stockQuantity"].(float64))

		line := strings.Repeat("─", 64)
		fmt.Printf("\n  %s%s%s\n", cBold, line, cReset)
		fmt.Printf("  %s  RESULTADO DO RACE CONDITION%s\n", cBold, cReset)
		fmt.Printf("  %s%s%s\n", cBold, line, cReset)
		fmt.Printf("  %-36s %s%d%s\n", "Estoque inicial:", cCyan, stock, cReset)
		fmt.Printf("  %-36s %d\n", "Requests disparados:", n)
		fmt.Printf("  %-36s %s%d%s\n", "Reservas aceitas (HTTP 2xx):", cGreen, successCount, cReset)
		fmt.Printf("  %-36s %s%d%s\n", "Reservas rejeitadas (HTTP 4/5xx):", cRed, n-successCount, cReset)
		fmt.Printf("  %-36s %s%d%s  (esperado: %d)\n",
			"Estoque REAL no banco:", cRed, finalStock, cReset, stock-successCount)
		fmt.Printf("  %s%s%s\n", cBold, line, cReset)

		fmt.Println()
		if finalStock < 0 {
			bad("RACE CONDITION CONFIRMADA! Estoque negativo: %d", finalStock)
			bad("Isso significa que mais produtos foram vendidos do que existiam em estoque!")
		} else if successCount > stock {
			bad("RACE CONDITION! %d reservas aceitas mas apenas %d unidades disponíveis", successCount, stock)
		} else if finalStock == 0 && successCount == stock {
			good("Sem race condition desta vez. Tente novamente (o resultado varia).")
			info("Aumente -concurrency para aumentar a probabilidade de race condition.")
		} else {
			hint("Resultado inconclusivo. Tente com mais requests: -concurrency=%d", n*2)
		}

		fmt.Println()
		hint("Solução:  adicionar @Transactional + @Lock(LockModeType.PESSIMISTIC_WRITE)")
		hint("Ou usar:  Optimistic Locking com @Version na entidade Product")
	}

	printSummary("race condition", results)
}

// ═══════════════════════════════════════════════════════════════
// CENÁRIO 5: ESGOTAMENTO DO POOL DE CONEXÕES
// ═══════════════════════════════════════════════════════════════

func scenarioPoolExhaustion(n int) {
	banner("CENÁRIO 5: POOL EXHAUSTION — HikariCP maximum-pool-size=2")

	info("O inventory-service tem apenas 2 conexões no pool do banco de dados")
	info("Com %d requests simultâneos: conexões 3 a %d ficam na FILA esperando", n, n)
	info("Isso se manifesta como aumento progressivo de latência sob carga")
	fmt.Println()

	// Fase 1: Baseline — requests sequenciais
	section("Fase 1: Baseline — 5 requests sequenciais (sem contenção)")
	var baseResults []Result
	for i := 1; i <= 5; i++ {
		r := doRequest("GET", inventoryURL+"/products", nil)
		r.ID = i
		r.Label = fmt.Sprintf("seq#%d", i)
		baseResults = append(baseResults, r)
		printResult(r)
	}
	printSummary("baseline sequencial", baseResults)

	fmt.Println()

	// Fase 2: Carga — N requests simultâneos de escrita (reserve usa 2 ops no banco)
	section(fmt.Sprintf("Fase 2: %d requests simultâneos de ESCRITA (read+write por request)", n))
	info("Cada PUT /reserve faz findById + save → segura conexão por mais tempo")
	fmt.Println()

	// Cria produto com estoque alto para o teste
	newProduct := map[string]interface{}{
		"name":          fmt.Sprintf("Produto-PoolTest-%d", time.Now().UnixMilli()),
		"description":   "Para teste de pool",
		"price":         10.00,
		"stockQuantity": n * 10,
	}
	r := doRequest("POST", inventoryURL+"/products", newProduct)
	if !r.OK() {
		bad("Falha ao criar produto de teste: %s", r.Body)
		return
	}
	var created map[string]interface{}
	json.Unmarshal([]byte(r.Body), &created)
	productID := int64(created["id"].(float64))
	good("Produto de teste criado: ID=%d, estoque=%d", productID, n*10)
	fmt.Println()

	var mu sync.Mutex
	var loadResults []Result
	var wg sync.WaitGroup
	wallStart := time.Now()

	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			url := fmt.Sprintf("%s/products/%d/reserve", inventoryURL, productID)
			r := doRequest("PUT", url, reservePayload(1))
			r.ID = id
			r.Label = fmt.Sprintf("pool#%-3d", id)
			mu.Lock()
			loadResults = append(loadResults, r)
			mu.Unlock()
			printResult(r)
		}(i)
	}
	wg.Wait()
	wall := time.Since(wallStart)

	fmt.Printf("\n  Wall time total: %s%s%s\n", cCyan, wall.Round(time.Millisecond), cReset)
	printSummary(fmt.Sprintf("%d requests simultâneos (pool=2)", n), loadResults)

	// Comparação de distribuição de latência
	fmt.Println()
	section("Distribuição de latência — visualizando o efeito do pool")
	var allDurs []time.Duration
	for _, res := range loadResults {
		allDurs = append(allDurs, res.Duration)
	}
	sort.Slice(allDurs, func(i, j int) bool { return allDurs[i] < allDurs[j] })

	buckets := []struct {
		label string
		max   time.Duration
	}{
		{"< 50ms  (sem espera)", 50 * time.Millisecond},
		{"50-200ms", 200 * time.Millisecond},
		{"200-500ms", 500 * time.Millisecond},
		{"500ms-1s", 1 * time.Second},
		{"> 1s    (longa espera)", 9999 * time.Second},
	}

	for _, b := range buckets {
		count := 0
		for _, d := range allDurs {
			if d < b.max {
				count++
			}
		}
		bar := strings.Repeat("█", count)
		if count == 0 {
			bar = "·"
		}
		color := cGreen
		if strings.HasPrefix(b.label, ">") {
			color = cRed
		} else if strings.HasPrefix(b.label, "500") || strings.HasPrefix(b.label, "200") {
			color = cYellow
		}
		fmt.Printf("  %-28s %s%s%s %d\n", b.label, color, bar, cReset, count)
	}

	fmt.Println()
	hint("Com pool=2, requests que excedem 2 simultâneos ficam em fila (connection-timeout=30s)")
	hint("Solução: aumentar maximum-pool-size para 10-20 no inventory-service")
	hint("Regra geral: pool-size = (núcleos_cpu × 2) + spindle_count")
}

// ═══════════════════════════════════════════════════════════════
// HTTP SERVER
// ═══════════════════════════════════════════════════════════════

func startServer(port string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlPage)
	})

	mux.HandleFunc("/run/", func(w http.ResponseWriter, r *http.Request) {
		scenario := strings.TrimPrefix(r.URL.Path, "/run/")
		concurrency := 20
		if c := r.URL.Query().Get("concurrency"); c != "" {
			if n, err := strconv.Atoi(c); err == nil && n > 0 && n <= 200 {
				concurrency = n
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		switch scenario {
		case "health":
			fmt.Fprintln(w, "✓ Rodando health check... acompanhe os logs do servidor.")
			go scenarioHealthCheck()
		case "latency":
			fmt.Fprintln(w, "✓ Rodando diagnóstico de latência... acompanhe os logs do servidor.")
			go scenarioLatency()
		case "single":
			fmt.Fprintln(w, "✓ Criando pedido único... acompanhe os logs do servidor.")
			go scenarioSingleOrder()
		case "concurrent":
			fmt.Fprintf(w, "✓ Disparando %d pedidos simultâneos... acompanhe os logs.\n", concurrency)
			go scenarioConcurrentOrders(concurrency)
		case "race":
			fmt.Fprintf(w, "✓ Testando race condition com %d requests... acompanhe os logs.\n", concurrency)
			go scenarioRaceCondition(concurrency)
		case "pool":
			fmt.Fprintf(w, "✓ Testando pool exhaustion com %d requests... acompanhe os logs.\n", concurrency)
			go scenarioPoolExhaustion(concurrency)
		case "all":
			fmt.Fprintln(w, "✓ Rodando todos os cenários em sequência... acompanhe os logs.")
			go func() {
				scenarioHealthCheck()
				pause(2 * time.Second)
				scenarioLatency()
				pause(2 * time.Second)
				scenarioSingleOrder()
				pause(2 * time.Second)
				scenarioRaceCondition(concurrency)
				pause(2 * time.Second)
				scenarioPoolExhaustion(concurrency)
				pause(2 * time.Second)
				scenarioConcurrentOrders(concurrency)
				logLine("\n%s  Todos os cenários concluídos!%s\n", cBold+cGreen, cReset)
			}()
		default:
			http.Error(w, "Cenário desconhecido. Opções: health, latency, single, concurrent, race, pool, all", 400)
		}
	})

	banner("LOAD TESTER — MODO SERVIDOR HTTP")
	fmt.Printf("  URL:  %shttp://localhost:%s%s\n\n", cCyan, port, cReset)
	fmt.Printf("  Endpoints disponíveis:\n")
	fmt.Printf("    GET /\n")
	fmt.Printf("    GET /run/health\n")
	fmt.Printf("    GET /run/latency\n")
	fmt.Printf("    GET /run/single\n")
	fmt.Printf("    GET /run/concurrent?concurrency=20\n")
	fmt.Printf("    GET /run/race?concurrency=30\n")
	fmt.Printf("    GET /run/pool?concurrency=20\n")
	fmt.Printf("    GET /run/all?concurrency=20\n\n")
	fmt.Printf("  %sOs resultados aparecem neste terminal (stdout)%s\n\n", cYellow, cReset)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao iniciar servidor: %v\n", err)
		os.Exit(1)
	}
}

// ═══════════════════════════════════════════════════════════════
// MAIN
// ═══════════════════════════════════════════════════════════════

func main() {
	scenario := flag.String("scenario", "all", "Cenário: health|latency|single|concurrent|race|pool|all")
	concurrency := flag.Int("concurrency", 20, "Número de requests simultâneos (para concurrent/race/pool)")
	server := flag.Bool("server", false, "Iniciar em modo servidor HTTP")
	port := flag.String("port", "9090", "Porta do servidor HTTP (com -server)")
	flag.Parse()

	if *server {
		startServer(*port)
		return
	}

	switch *scenario {
	case "health":
		scenarioHealthCheck()
	case "latency":
		scenarioLatency()
	case "single":
		scenarioSingleOrder()
	case "concurrent":
		scenarioConcurrentOrders(*concurrency)
	case "race":
		scenarioRaceCondition(*concurrency)
	case "pool":
		scenarioPoolExhaustion(*concurrency)
	case "all":
		fmt.Printf("\n%s  ECOMMERCE LOAD TESTER — TODOS OS CENÁRIOS  (concurrency=%d)%s\n",
			cBold+cCyan, *concurrency, cReset)
		scenarioHealthCheck()
		pause(2 * time.Second)
		scenarioLatency()
		pause(2 * time.Second)
		scenarioSingleOrder()
		pause(2 * time.Second)
		scenarioRaceCondition(*concurrency)
		pause(2 * time.Second)
		scenarioPoolExhaustion(*concurrency)
		pause(2 * time.Second)
		scenarioConcurrentOrders(*concurrency)
		fmt.Printf("\n%s  Todos os cenários concluídos!%s\n\n", cBold+cGreen, cReset)
	default:
		fmt.Fprintf(os.Stderr, "Cenário desconhecido: %s\n", *scenario)
		fmt.Fprintln(os.Stderr, "Opções: health, latency, single, concurrent, race, pool, all")
		os.Exit(1)
	}
}

// ═══════════════════════════════════════════════════════════════
// HTML PAGE (embutida no binário)
// ═══════════════════════════════════════════════════════════════

const htmlPage = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<title>Ecommerce Load Tester</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: 'Courier New', monospace;
    background: #0d1117; color: #c9d1d9;
    padding: 2rem; max-width: 900px; margin: 0 auto;
  }
  h1 { color: #58a6ff; font-size: 1.6rem; margin-bottom: 0.5rem; }
  .subtitle { color: #8b949e; margin-bottom: 2rem; font-size: 0.9rem; }
  h2 { color: #79c0ff; font-size: 1rem; margin-bottom: 0.75rem; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
  .card {
    background: #161b22; border: 1px solid #30363d;
    border-radius: 8px; padding: 1.25rem;
  }
  .card.full { grid-column: 1 / -1; }
  .btn {
    background: #21262d; color: #58a6ff;
    border: 1px solid #388bfd66;
    padding: 0.45rem 1.2rem; border-radius: 6px;
    cursor: pointer; font-family: inherit; font-size: 0.85rem;
    margin: 0.2rem 0.2rem 0.2rem 0; transition: all .15s;
  }
  .btn:hover { background: #388bfd22; border-color: #58a6ff; }
  .btn.danger { color: #f85149; border-color: #f8514966; }
  .btn.danger:hover { background: #f8514922; }
  .btn.run-all { color: #3fb950; border-color: #3fb95066; }
  .btn.run-all:hover { background: #3fb95022; }
  .note { color: #8b949e; font-size: 0.8rem; margin-top: 0.6rem; line-height: 1.5; }
  .warn { color: #d29922; }
  .err  { color: #f85149; }
  .ok   { color: #3fb950; }
  .concurrency-row {
    background: #161b22; border: 1px solid #30363d;
    border-radius: 8px; padding: 1.25rem;
    margin-bottom: 1rem;
    display: flex; align-items: center; gap: 1rem;
  }
  input[type=number] {
    background: #0d1117; color: #c9d1d9;
    border: 1px solid #388bfd66; padding: 0.4rem 0.6rem;
    border-radius: 6px; width: 90px; font-family: inherit; font-size: 0.9rem;
  }
  #status {
    margin-top: 1rem; padding: 0.75rem 1rem;
    background: #161b22; border: 1px solid #30363d;
    border-radius: 6px; font-size: 0.85rem; color: #3fb950;
    display: none;
  }
  code { background: #0d1117; padding: 0.15rem 0.4rem; border-radius: 4px; color: #79c0ff; font-size: 0.8rem; }
</style>
</head>
<body>

<h1>⚡ Ecommerce Load Tester</h1>
<p class="subtitle">
  Acione os cenários abaixo — os resultados aparecem no <strong>terminal</strong> onde o load-tester está rodando.<br>
  Serviços: order :8080 | inventory :8081 | invoice :8082 | notification :8083
</p>

<div class="concurrency-row">
  <label>Concorrência:
    <input type="number" id="n" value="20" min="1" max="200">
  </label>
  <span class="note">Número de requests simultâneos para os cenários de carga</span>
</div>

<div id="status"></div>

<div class="grid">

  <div class="card">
    <h2>🔍 Diagnóstico</h2>
    <button class="btn" onclick="run('health')">Health Check</button>
    <button class="btn" onclick="run('latency')">Latência por Serviço</button>
    <p class="note">
      Health: verifica se todos os serviços respondem.<br>
      Latência: mede invoice (~2s) e notification (~1.5s) isoladamente.
    </p>
  </div>

  <div class="card">
    <h2>🐌 Pedido Único</h2>
    <button class="btn" onclick="run('single')">Criar 1 Pedido</button>
    <p class="note warn">
      ⚠ Demonstra a latência acumulada da cadeia síncrona.<br>
      Esperado: ~3.5s por pedido (invoice + notification bloqueantes).
    </p>
  </div>

  <div class="card">
    <h2>⚡ Race Condition no Estoque</h2>
    <button class="btn danger" onclick="runN('race')">Testar Race Condition</button>
    <p class="note err">
      ✗ Cria produto com N/2 estoque, dispara N reservas simultâneas.<br>
      Sem <code>@Transactional</code> + locking: estoque pode ficar negativo!
    </p>
  </div>

  <div class="card">
    <h2>🔒 Pool de Conexões</h2>
    <button class="btn danger" onclick="runN('pool')">Testar Pool Exhaustion</button>
    <p class="note warn">
      ⚠ Inventory tem <code>maximum-pool-size=2</code>.<br>
      N requests simultâneos enfileiram e a latência sobe drasticamente.
    </p>
  </div>

  <div class="card full">
    <h2>🏎️ Pedidos Simultâneos — Thread Starvation</h2>
    <button class="btn danger" onclick="runN('concurrent')">Disparar N Pedidos</button>
    <p class="note err">
      ✗ Cada pedido bloqueia 1 thread do Tomcat por ~3.5s.<br>
      Com N grande: threads esgotam → novos requests ficam em fila ou falham.<br>
      Solução futura: invoice e notification assíncronos via mensageria (Kafka/RabbitMQ).
    </p>
  </div>

  <div class="card full" style="border-color: #3fb95044;">
    <h2>🚀 Rodar Todos os Cenários</h2>
    <button class="btn run-all" onclick="runN('all')">Rodar Tudo em Sequência</button>
    <p class="note ok">
      Executa: health → latency → single → race → pool → concurrent<br>
      Pode demorar vários minutos dependendo da concorrência configurada.
    </p>
  </div>

</div>

<script>
  function getN() { return document.getElementById('n').value; }

  function showStatus(msg, ok) {
    const el = document.getElementById('status');
    el.style.display = 'block';
    el.style.color = ok ? '#3fb950' : '#f85149';
    el.textContent = msg;
  }

  function run(sc) {
    showStatus('Acionando cenário "' + sc + '"...', true);
    fetch('/run/' + sc)
      .then(r => r.text())
      .then(t => showStatus('✓ ' + t.trim() + ' — veja o terminal!', true))
      .catch(e => showStatus('✗ Erro: ' + e, false));
  }

  function runN(sc) {
    const n = getN();
    showStatus('Acionando cenário "' + sc + '" com concurrency=' + n + '...', true);
    fetch('/run/' + sc + '?concurrency=' + n)
      .then(r => r.text())
      .then(t => showStatus('✓ ' + t.trim() + ' — veja o terminal!', true))
      .catch(e => showStatus('✗ Erro: ' + e, false));
  }
</script>

</body>
</html>`
