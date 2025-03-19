package wargaming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type CaptchaSolver interface {
	Solve(imageBase64 string) (string, error)
}

type Client struct {
	httpClient    *http.Client
	captchaSolver CaptchaSolver
	solveCaptcha  bool
}

func NewClient(proxy *url.URL, captchaSolver CaptchaSolver, solveCaptcha bool) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	return &Client{
		httpClient: &http.Client{
			Jar: jar,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxy),
			},
			Timeout: time.Second * 10,
		},
		solveCaptcha:  solveCaptcha,
		captchaSolver: captchaSolver,
	}
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) GetCSRF() (string, error) {
	u, _ := url.Parse("https://wargaming.net/id/signin/")
	cookies := c.httpClient.Jar.Cookies(u)

	for _, cookie := range cookies {
		if cookie.Name == CsrfCookieName {
			return cookie.Value, nil
		}
	}

	req, err := http.NewRequest(http.MethodGet, "https://wargaming.net/id/state.json", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(`Referer`, `https://wargaming.net/id/signin/`)
	req.Header.Set(`x-requested-with`, `XMLHttpRequest`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == CsrfCookieName {
			return cookie.Value, nil
		}
	}

	return "", ErrCSRFNotFound
}

type Account struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	GameRealm string `json:"game_realm"`
	IsDefault bool   `json:"is_default"`
}

func (c *Client) SignIn(email, password string) ([]Account, error) {
	csrf, err := c.GetCSRF()
	if err != nil {
		return nil, err
	}

	pow, err := c.GetPow()
	if err != nil {
		return nil, err
	}

	powSolution, err := SolvePOW(pow)
	if err != nil {
		if c.solveCaptcha {
			captcha, err := c.GetCaptcha()
			if err != nil {
				return nil, err
			}

			captchaSolution, err := c.captchaSolver.Solve(captcha.ImageBase64)
			if err != nil {
				return nil, err
			}

			form := url.Values{}
			form.Add("login", email)
			form.Add("password", password)
			form.Add("captcha", captchaSolution)

			req, err := http.NewRequest(http.MethodPost, "https://wargaming.net/id/signin/process/?type=captcha", strings.NewReader(form.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set(`content-type`, `application/x-www-form-urlencoded`)
			req.Header.Set(`referer`, `https://wargaming.net/id/signin/`)
			req.Header.Set(`x-csrftoken`, csrf)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			location := resp.Header.Get("Location")

			if location != "" {
				req, err := http.NewRequest(http.MethodGet, location, nil)
				if err != nil {
					return nil, err
				}

				resp, err := c.httpClient.Do(req)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()

				return c.parseSignInResponse(resp)
			}

			return c.parseSignInResponse(resp)
		} else {
			return nil, err
		}
	}

	form := url.Values{}
	form.Add("pow", fmt.Sprint(powSolution))
	form.Add("login", email)
	form.Add("password", password)
	form.Add("captcha", "")

	req, err := http.NewRequest(http.MethodPost, "https://wargaming.net/id/signin/process/?type=pow", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set(`content-type`, `application/x-www-form-urlencoded`)
	req.Header.Set(`referer`, `https://wargaming.net/id/signin/`)
	req.Header.Set(`x-csrftoken`, csrf)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")

	if location != "" {
		req, err := http.NewRequest(http.MethodGet, location, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return c.parseSignInResponse(resp)
	}

	return c.parseSignInResponse(resp)
}

func (c *Client) parseSignInResponse(resp *http.Response) ([]Account, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var responseMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return nil, fmt.Errorf("failed to decode sign-in response: %w", err)
	}

	if errorsRaw, ok := responseMap["errors"]; ok {
		var errors struct {
			All     []string `json:"__all__"`
			Captcha []string `json:"captcha"`
		}
		if err := json.Unmarshal(errorsRaw, &errors); err != nil {
			return nil, fmt.Errorf("failed to parse errors: %w", err)
		}

		if len(errors.All) > 0 {
			if errors.All[0] == "invalid_credentials" {
				return nil, ErrInvalidCredentials
			} else if errors.All[0] == "account_choice" {
				if extrasRaw, ok := responseMap["extras"]; ok {
					var extras struct {
						Accounts        []Account `json:"accounts"`
						IsKoreanAccount bool      `json:"is_korean_account"`
						IsMinorAccount  bool      `json:"is_minor_account"`
						NextURL         string    `json:"next_url"`
					}

					if err := json.Unmarshal(extrasRaw, &extras); err != nil {
						return nil, fmt.Errorf("failed to parse extras: %w", err)
					}

					return extras.Accounts, nil
				}
			} else if errors.All[0] == "merge_required" {
				return nil, ErrMergeRequired
			}

			return nil, fmt.Errorf("authentication failed: %s", errors.All[0])
		} else if len(errors.Captcha) > 0 {
			if errors.Captcha[0] == "invalid" {
				return nil, ErrInvalidCaptcha
			}
		}
	}

	return nil, nil
}

func (c *Client) ChooseAccount(id int, gameRealm string) error {
	form := url.Values{}
	form.Add("account_id", fmt.Sprint(id))
	form.Add("account_game_realm", gameRealm)
	form.Add("next", "/")

	req, err := http.NewRequest(http.MethodPost, "https://wargaming.net/id/signin/persona/accounts/select/", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set(`content-type`, `application/x-www-form-urlencoded`)
	req.Header.Set(`referer`, `https://wargaming.net/id/signin/`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) getAccessToken() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://wargaming.net/id/sessionwidget/token/?response_type=token&client_id=common_menu&scope=openid&origin=https://wargaming.net", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	query, err := url.ParseQuery(resp.Request.URL.Fragment)
	if err != nil {
		return "", err
	}

	return query.Get("access_token"), nil
}

type AccountInfo struct {
	Sub       int    `json:"sub"`
	ID        int    `json:"id"`
	GameRealm string `json:"game_realm"`
	Nickname  string `json:"nickname"`
}

func (c *Client) GetAccountInfo() (AccountInfo, error) {
	accessToken, err := c.getAccessToken()
	if err != nil {
		return AccountInfo{}, err
	}

	form := url.Values{}
	form.Add("fields", "id,nickname,game_realm")

	req, err := http.NewRequest(http.MethodPost, "https://wargaming.net/id/api/v2/account/info/", strings.NewReader(form.Encode()))
	if err != nil {
		return AccountInfo{}, err
	}
	req.Header.Set(`content-type`, `application/x-www-form-urlencoded`)
	req.Header.Set(`authorization`, `Bearer `+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return AccountInfo{}, err
	}
	defer resp.Body.Close()

	location := resp.Header.Get("location")
	if location != "" {
		req, err = http.NewRequest(http.MethodGet, location, nil)
		if err != nil {
			return AccountInfo{}, err
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return AccountInfo{}, err
		}
		defer resp.Body.Close()
	}

	var accountInfo AccountInfo
	if err := json.NewDecoder(resp.Body).Decode(&accountInfo); err != nil {
		return AccountInfo{}, err
	}

	return accountInfo, nil
}

type GetVehiclesRequest struct {
	BattleType       string `json:"battle_type"`
	OnlyInGarage     bool   `json:"only_in_garage"`
	SpaID            int    `json:"spa_id"`
	Premium          []int  `json:"premium"`
	CollectorVehicle []int  `json:"collector_vehicle"`
	Nation           []int  `json:"nation"`
	Role             []int  `json:"role"`
	Type             []int  `json:"type"`
	Tier             []int  `json:"tier"`
	Language         string `json:"language"`
}

type VehicleData struct {
	VehicleCD             int     `json:"vehicle_cd"`
	TechName              string  `json:"tech_name"`
	Premium               int     `json:"premium"`
	CollectorVehicle      int     `json:"collector_vehicle"`
	Nation                string  `json:"nation"`
	Type                  string  `json:"type"`
	Role                  string  `json:"role"`
	Name                  string  `json:"name"`
	Tier                  int     `json:"tier"`
	FragsDeathsRatio      float64 `json:"frags_deaths_ratio"`
	MarkOfMastery         int     `json:"markOfMastery"`
	DamageDealt           int     `json:"damage_dealt"`
	WinsRatio             float64 `json:"wins_ratio"`
	XpPerBattleAvg        float64 `json:"xp_per_battle_average"`
	MarksOnGun            int     `json:"marksOnGun"`
	DamageReceived        int     `json:"damage_received"`
	DamagePerBattleAvg    float64 `json:"damage_per_battle_average"`
	HitsCount             int     `json:"hits_count"`
	FragsCount            int     `json:"frags_count"`
	DmgDealtReceivedRatio float64 `json:"damage_dealt_received_ratio"`
	FragsPerBattleAvg     float64 `json:"frags_per_battle_average"`
	BattlesCount          int     `json:"battles_count"`
	SurvivedBattles       int     `json:"survived_battles"`
	XpAmount              int     `json:"xp_amount"`
	WinsCount             int     `json:"wins_count"`
}

type VehiclesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Meta struct {
			MarksOnGun map[string]map[string]string `json:"marks_on_gun"`
		} `json:"meta"`
		Data       [][]interface{} `json:"data"`
		Parameters []string        `json:"parameters"`
	} `json:"data"`
}

func (c *Client) GetVehicles(id int, gameRealm string) ([]VehicleData, error) {
	reqBody := GetVehiclesRequest{
		BattleType:       "random",
		OnlyInGarage:     false,
		SpaID:            id,
		Premium:          []int{0, 1},
		CollectorVehicle: []int{0, 1},
		Nation:           []int{},
		Role:             []int{},
		Type:             []int{},
		Tier:             []int{},
		Language:         "en",
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	host := "worldoftanks.com"
	switch gameRealm {
	case "eu":
		host = "worldoftanks.eu"
	case "us":
		host = "worldoftanks.com"
	case "sg":
		host = "worldoftanks.asia"
	}

	url := fmt.Sprintf("https://%s/wotup/profile/vehicles/list/", host)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response VehiclesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode vehicles response: %w", err)
	}

	if response.Status != "ok" {
		return nil, fmt.Errorf("API returned non-ok status: %s", response.Status)
	}

	vehicles := make([]VehicleData, 0, len(response.Data.Data))

	for _, vehicleData := range response.Data.Data {
		paramMap := make(map[string]interface{})

		for i, paramName := range response.Data.Parameters {
			if i < len(vehicleData) {
				paramMap[paramName] = vehicleData[i]
			}
		}

		jsonData, err := json.Marshal(paramMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal vehicle data: %w", err)
		}

		var vehicle VehicleData
		if err := json.Unmarshal(jsonData, &vehicle); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vehicle data: %w", err)
		}

		vehicles = append(vehicles, vehicle)
	}

	return vehicles, nil
}
