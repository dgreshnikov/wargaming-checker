package captchasolver

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrCaptchaNotReady = errors.New("captcha not ready")
)

type CaptchaSolver struct {
	httpClient *http.Client
	host       string
	apiKey     string
}

func (s *CaptchaSolver) CreateTask(imageBase64 string) (string, error) {
	form := url.Values{}
	form.Add("body", imageBase64)
	form.Add("method", "base64")

	req, err := http.NewRequest(http.MethodPost, s.host+"/in.php", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	if !strings.HasPrefix(bodyStr, "OK|") {
		return "", errors.New(bodyStr)
	}

	parts := strings.Split(bodyStr, "|")

	return parts[1], nil
}

func (s *CaptchaSolver) GetResult(id string) (string, error) {
	form := url.Values{}
	form.Add("action", "get")
	form.Add("id", id)

	req, err := http.NewRequest(http.MethodPost, s.host+"/res.php", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	if bodyStr == "CAPCHA_NOT_READY" {
		return "", ErrCaptchaNotReady
	}

	if !strings.HasPrefix(bodyStr, "OK|") {
		return "", errors.New(bodyStr)
	}

	parts := strings.Split(bodyStr, "|")
	return parts[1], nil
}

func (s *CaptchaSolver) WaitForResult(id string, maxAttempts int, delay time.Duration) (string, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := s.GetResult(id)
		if err == nil {
			return result, nil
		}

		if err != ErrCaptchaNotReady {
			return "", err
		}

		time.Sleep(delay)
	}

	return "", errors.New("max attempts reached waiting for captcha result")
}

func NewSolver(host string, apiKey string) (*CaptchaSolver, error) {
	return &CaptchaSolver{
		httpClient: &http.Client{},
		host:       host,
		apiKey:     apiKey,
	}, nil
}

func (s *CaptchaSolver) Solve(imageBase64 string) (string, error) {
	taskId, err := s.CreateTask(imageBase64)
	if err != nil {
		return "", err
	}

	result, err := s.WaitForResult(taskId, 10, time.Millisecond*100)
	if err != nil {
		return "", err
	}

	return result, nil
}
