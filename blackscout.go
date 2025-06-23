package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Config struct {
	TargetURL string
	Threads   int
	DelayMs   int
}

type Crawler struct {
	BaseURL     *url.URL
	Visited     map[string]bool
	Mutex       sync.Mutex
	Results     []string
	ThreadLimit chan struct{}
	DelayMs     int
}

var totalRequests int
var requestsMutex sync.Mutex

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func banner() {
	fmt.Println(`
 ▄▄▄▄    ██▓    ▄▄▄       ▄████▄   ██ ▄█▀  ██████  ▄████▄   ▒█████   █    ██ ▄▄▄█████▓
▓█████▄ ▓██▒   ▒████▄    ▒██▀ ▀█   ██▄█▒ ▒██    ▒ ▒██▀ ▀█  ▒██▒  ██▒ ██  ▓██▒▓  ██▒ ▓▒
▒██▒ ▄██▒██░   ▒██  ▀█▄  ▒▓█    ▄ ▓███▄░ ░ ▓██▄   ▒▓█    ▄ ▒██░  ██▒▓██  ▒██░▒ ▓██░ ▒░
▒██░█▀  ▒██░   ░██▄▄▄▄██ ▒▓▓▄ ▄██▒▓██ █▄   ▒   ██▒▒▓▓▄ ▄██▒▒██   ██░▓▓█  ░██░░ ▓██▓ ░ 
░▓█  ▀█▓░██████▒▓█   ▓██▒▒ ▓███▀ ░▒██▒ █▄▒██████▒▒▒ ▓███▀ ░░ ████▓▒░▒▒█████▓   ▒██▒ ░ 
░▒▓███▀▒░ ▒░▓  ░▒▒   ▓▒█░░ ░▒ ▒  ░▒ ▒▒ ▓▒▒ ▒▓▒ ▒ ░░ ░▒ ▒  ░░ ▒░▒░▒░ ░▒▓▒ ▒ ▒   ▒ ░░   
▒░▒   ░ ░ ░ ▒  ░ ▒   ▒▒ ░  ░  ▒   ░ ░▒ ▒░░ ░▒  ░ ░  ░  ▒     ░ ▒ ▒░ ░░▒░ ░ ░     ░    
 ░    ░   ░ ░    ░   ▒   ░        ░ ░░ ░ ░  ░  ░  ░        ░ ░ ░ ▒   ░░░ ░ ░   ░      
 ░          ░  ░     ░  ░░ ░      ░  ░         ░  ░ ░          ░ ░     ░              
      ░                  ░                        ░                                                                                                                                                               
        BlackScout - Author: Nexus | Team: The Project Nexus
	`)
}

func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func getConfig() Config {
	var config Config

	config.TargetURL = readInput("Digite a URL alvo (ex: https://site.com): ")

	for {
		threads := readInput("Número de threads (ex: 10): ")
		val, err := strconv.Atoi(threads)
		if err == nil && val > 0 {
			config.Threads = val
			break
		} else {
			fmt.Println("Por favor, insira um número válido de threads.")
		}
	}

	for {
		delay := readInput("Delay entre requisições (em ms, ex: 300): ")
		val, err := strconv.Atoi(delay)
		if err == nil && val >= 0 {
			config.DelayMs = val
			break
		} else {
			fmt.Println("Por favor, insira um número válido de delay.")
		}
	}

	return config
}

func NewCrawler(target string, threads int, delay int) (*Crawler, error) {
	parsedURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	return &Crawler{
		BaseURL:     parsedURL,
		Visited:     make(map[string]bool),
		ThreadLimit: make(chan struct{}, threads),
		DelayMs:     delay,
	}, nil
}

func (c *Crawler) normalize(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}

	if u.IsAbs() {
		if u.Host == c.BaseURL.Host {
			return u.String()
		}
		return ""
	}

	resolved := c.BaseURL.ResolveReference(u)
	if resolved.Host == c.BaseURL.Host {
		return resolved.String()
	}

	return ""
}

func (c *Crawler) fetchAndParse(target string, wg *sync.WaitGroup) {
	defer wg.Done()
	c.ThreadLimit <- struct{}{}
	defer func() { <-c.ThreadLimit }()
	time.Sleep(time.Duration(rand.Intn(c.DelayMs)+1) * time.Millisecond)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", target, nil)
	req.Header.Set("User-Agent", randomUserAgent())

	incrementRequests()

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(colorText("\n[ERRO] Falha ao acessar: "+target, "red"))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return
	}

	tokenizer := html.NewTokenizer(resp.Body)
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}

		t := tokenizer.Token()
		if t.Type == html.StartTagToken {
			for _, attr := range t.Attr {
				if attr.Key == "href" || attr.Key == "src" || attr.Key == "action" {
					link := c.normalize(attr.Val)
					if link != "" {
						c.Mutex.Lock()
						if !c.Visited[link] {
							c.Visited[link] = true
							c.Results = append(c.Results, link)
							wg.Add(1)
							go c.fetchAndParse(link, wg)
						}
						c.Mutex.Unlock()
					}
				}
			}
		}
	}
}

func (c *Crawler) Start() []string {
	var wg sync.WaitGroup
	c.Mutex.Lock()
	c.Visited[c.BaseURL.String()] = true
	c.Results = append(c.Results, c.BaseURL.String())
	c.Mutex.Unlock()
	wg.Add(1)
	go c.fetchAndParse(c.BaseURL.String(), &wg)
	wg.Wait()
	return c.Results
}

func incrementRequests() {
	requestsMutex.Lock()
	totalRequests++
	requestsMutex.Unlock()
}

func getTotalRequests() int {
	requestsMutex.Lock()
	defer requestsMutex.Unlock()
	return totalRequests
}

func showLiveProgress(start time.Time) {
	for {
		time.Sleep(1 * time.Second)
		elapsed := time.Since(start).Seconds()
		rps := float64(getTotalRequests()) / elapsed
		fmt.Printf("\r%s Total Requests: %d | RPS: %.2f %s", colorText("[Progresso]", "yellow"), getTotalRequests(), rps, colorText("", "reset"))
		if elapsed > 3600 {
			break
		}
	}
}

func randomUserAgent() string {
	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/114.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/100.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
	}
	return agents[rand.Intn(len(agents))]
}

func colorText(text string, color string) string {
	colors := map[string]string{
		"red":    "\033[31m",
		"green":  "\033[32m",
		"yellow": "\033[33m",
		"reset":  "\033[0m",
	}

	return colors[color] + text + colors["reset"]
}

func displayResults(results []string) {
	fmt.Println("\n===================== Endpoints Encontrados =====================")
	fmt.Printf("%-5s %-70s\n", "ID", "URL")
	fmt.Println("=================================================================")

	for i, url := range results {
		if len(url) > 68 {
			url = url[:65] + "..."
		}
		fmt.Printf("%-5d %-70s\n", i+1, url)
	}

	fmt.Println("=================================================================")
}

func exportResults(results []string) {
	file, err := os.Create("endpoints.txt")
	if err != nil {
		fmt.Println("Erro ao criar o arquivo:", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, url := range results {
		writer.WriteString(url + "\n")
	}
	writer.Flush()
	fmt.Println(colorText("\n[SUCESSO] Resultados exportados para endpoints.txt", "green"))
}

func askToExport(results []string) {
	input := readInput("\nDeseja exportar os endpoints para um arquivo .txt? (S/N): ")
	if strings.ToLower(input) == "s" {
		exportResults(results)
	} else {
		fmt.Println("Exportação ignorada.")
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	clearScreen()
	banner()
	config := getConfig()

	crawler, err := NewCrawler(config.TargetURL, config.Threads, config.DelayMs)
	if err != nil {
		fmt.Println("Erro ao iniciar o crawler:", err)
		return
	}

	start := time.Now()
	go showLiveProgress(start)

	fmt.Println("\nIniciando varredura...")
	results := crawler.Start()

	duration := time.Since(start).Seconds()

	displayResults(results)
	fmt.Printf("\nTotal de endpoints encontrados: %d\n", len(results))
	fmt.Printf("Tempo total da varredura: %.2f segundos\n", duration)

	askToExport(results)
}
