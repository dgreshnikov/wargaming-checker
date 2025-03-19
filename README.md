# Wargaming Account Checker

A tool for checking Wargaming accounts.

## Usage

```bash
go run . -threads 100 -attempts 5 -base .\base.txt -proxies .\proxies.txt
```

# Flags

```bash
  -attempts int
        Maximum number of attempts per account (default 10)
  -base string
        Path to the base file with email:password combinations (default "base.txt")
  -captcha-key string
        API key for the captcha solver service (optional)
  -captcha-url string
        URL for the captcha solver service (optional)
  -proxies string
        Path to the proxies file (default "proxies.txt")
  -results string
        Directory to store results (default "results")
  -threads int
        Number of concurrent threads (default 250)
```