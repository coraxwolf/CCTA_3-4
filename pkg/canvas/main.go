package canvas

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type APIManager struct {
	client                *http.Client
	logger                *slog.Logger
	maxRateLimit          int
	rateLimitRemaining    float64
	averageRateCost       float64
	requestSendCount      int
	responseReceivedCount int
	config                APIConfig
}

type APIConfig struct {
	Token   string
	BaseURL string
}

func NewAPI(
	logger *slog.Logger,
	token, baseURL string,
	rateLimitMax int,
	readTimeout int,
) *APIManager {
	if readTimeout <= 30 {
		logger.Warn("Read Timeout is set too low, setting to a minimum of 60 seconds")
		readTimeout = 60
	}
	client := &http.Client{
		Timeout: time.Duration(readTimeout) * time.Second,
	}
	cfg := APIConfig{
		Token:   token,
		BaseURL: baseURL,
	}
	return &APIManager{
		client:                client,
		logger:                logger,
		maxRateLimit:          rateLimitMax,
		rateLimitRemaining:    float64(rateLimitMax),
		averageRateCost:       0.0,
		config:                cfg,
		requestSendCount:      0,
		responseReceivedCount: 0,
	}
}

func (api *APIManager) Get(endpoint string) (*http.Response, error) {
	var delay time.Duration
	api.requestSendCount++
	previousCost := api.averageRateCost
	req, err := http.NewRequest("GET", api.config.BaseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+api.config.Token)

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}

	api.responseReceivedCount++
	// Get Rate Limit Information
	limit, err := strconv.ParseFloat(resp.Header.Get("RateLimit-Remaining"), 64)
	if err != nil {
		api.logger.Error("failed to parse RateLimit-Remaining header", "error", err)
		limit = float64(api.maxRateLimit / 2) // set to 50% since we do not know the actual limit
	}
	cost, err := strconv.ParseFloat(resp.Header.Get("Request-Cost"), 64)
	if err != nil {
		api.logger.Error("failed to parse Request-Cost header", "error", err)
		cost = api.averageRateCost // use the average rate cost if we cannot parse the header
	}
	if limit < float64(api.maxRateLimit) {
		api.rateLimitRemaining = limit
	} else {
		api.rateLimitRemaining = float64(api.maxRateLimit)
	}
	if cost > 0 {
		api.averageRateCost = api.averageRateCost + (cost-api.averageRateCost)/float64(api.requestSendCount)
	} else {
		api.logger.Warn("Request Cost is zero, cannot update average rate cost", "cost", cost)
	}
	// Plan Allowance for Rate Limit to Recharge and avoid hitting the limit
	if api.rateLimitRemaining <= float64(api.maxRateLimit)*0.25 {
		api.logger.Warn("Rate Limit is Extremely Low!! Under 25% of limit", "remaining", api.rateLimitRemaining)
		delay = time.Duration(api.maxRateLimit) * time.Second / 2 // Sleep for half the value of the max rate limit
	} else if api.rateLimitRemaining <= float64(api.maxRateLimit)*0.5 {
		api.logger.Warn("Rate Limit is Low Below 50%", "remaining", api.rateLimitRemaining)
		delay = time.Duration(api.maxRateLimit) * time.Second / 4 // Sleep for a quarter of the value of the max rate limit
	} else if api.rateLimitRemaining <= float64(api.maxRateLimit)*0.75 {
		api.logger.Info("Rate Limit is moderate between 50% and 75%", "remaining", api.rateLimitRemaining)
		delay = time.Duration(api.maxRateLimit) * time.Second / 8 // Sleep for an eighth of the value of the max rate limit
	} else {
		api.logger.Info("Rate Limit is healthy above 75%", "remaining", api.rateLimitRemaining)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		api.logger.Warn("Rate limit exceeded", "remaining", resp.Header.Get("X-RateLimit-Remaining"))
		delay = time.Duration(5) * time.Minute // Sleep for 5 minutes if rate limit is exceeded
	}

	// Extra Check if cost of last request is more than 20% higher than the average cost pause for 30 seconds
	if previousCost > 0 && cost > previousCost*1.2 {
		api.logger.Warn("Request cost is a significant increase from previous requests, adding an extra delay", "previousAverageCost", previousCost, "currentCost", cost)
		delay = 30 * time.Second // Sleep for 30 seconds if the cost is significantly higher
	}
	if delay > 0 {
		jitter := time.Duration(rand.Int63n(int64(delay)/4)) * time.Millisecond // Add jitter to the delay
		api.logger.Info("Delaying request due to rate limit or cost increase", "delay", delay)
		time.Sleep(delay + jitter) // Add jitter to make sure every delay is slightly different from the others
	}
	return resp, nil
}

func (api *APIManager) Post(endpoint string, body []byte) (*http.Response, error) {
	return nil, fmt.Errorf("post method not implemented yet")
}

func (api *APIManager) Put(endpoint string, body []byte) (*http.Response, error) {
	return nil, fmt.Errorf("put method not implemented yet")
}

func (api *APIManager) Delete(endpoint string) (*http.Response, error) {
	return nil, fmt.Errorf("delete method not implemented yet")
}
