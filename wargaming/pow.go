package wargaming

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/sha3"
)

type POWTask struct {
	RandomString string `json:"random_string"`
	Complexity   int    `json:"complexity"`
	Timestamp    int    `json:"timestamp"`
	Algorithm    struct {
		Name      string `json:"name"`
		Version   int    `json:"version"`
		Resourse  string `json:"resourse"`
		Extension string `json:"extension"`
	} `json:"algorithm"`
	Type string `json:"type"`
}

func (c *Client) GetPow() (POWTask, error) {
	req, err := http.NewRequest(http.MethodGet, "https://wargaming.net/id/signin/challenge/?type=pow", nil)
	if err != nil {
		return POWTask{}, err
	}
	req.Header.Set(`Referer`, `https://wargaming.net/id/signin/`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return POWTask{}, err
	}
	defer resp.Body.Close()

	var powResponse struct {
		Pow POWTask `json:"pow"`
	}
	err = json.NewDecoder(resp.Body).Decode(&powResponse)
	if err != nil {
		return POWTask{}, err
	}

	return powResponse.Pow, nil
}

func SolvePOW(pow POWTask) (int, error) {
	if pow.Complexity >= 10 {
		return 0, ErrPOWTooHard
	}

	stamp := fmt.Sprintf("%d:%d:%d:%s:%s:%s", pow.Algorithm.Version, pow.Complexity, pow.Timestamp, pow.Algorithm.Resourse, pow.Algorithm.Extension, pow.RandomString)
	prefix := strings.Repeat("0", pow.Complexity)

	for i := 1; i < 30_000; i++ {
		hash := sha3.NewLegacyKeccak512()
		hash.Write([]byte(fmt.Sprintf("%s:%d", stamp, i)))
		hashBytes := hash.Sum(nil)
		if strings.HasPrefix(hex.EncodeToString(hashBytes), prefix) {
			return i, nil
		}
	}

	return 0, ErrPOWTooHard
}
