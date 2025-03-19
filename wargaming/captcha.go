package wargaming

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

type Captcha struct {
	ImageBase64 string
	Token       string
}

func (c *Client) GetCaptcha() (Captcha, error) {
	req, err := http.NewRequest(http.MethodGet, "https://wargaming.net/id/signin/challenge/?type=captcha", nil)
	if err != nil {
		return Captcha{}, err
	}
	req.Header.Set(`Referer`, `https://wargaming.net/id/signin/`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Captcha{}, err
	}
	defer resp.Body.Close()

	var captchaResponse struct {
		Captcha struct {
			Token string `json:"token"`
			Url   string `json:"url"`
		}
	}
	err = json.NewDecoder(resp.Body).Decode(&captchaResponse)
	if err != nil {
		return Captcha{}, err
	}

	captchaImage, err := c.getCaptchaImageBase64("wargaming.net", captchaResponse.Captcha.Url)
	if err != nil {
		return Captcha{}, err
	}

	return Captcha{
		ImageBase64: captchaImage,
		Token:       captchaResponse.Captcha.Token,
	}, nil
}

func (c *Client) getCaptchaImageBase64(host string, url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://"+host+url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(imageBytes), nil
}
