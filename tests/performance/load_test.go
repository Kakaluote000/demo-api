package performance

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrentUsers := 100
	requestsPerUser := 100
	baseURL := "http://localhost:8080"

	var wg sync.WaitGroup
	start := time.Now()
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			for j := 0; j < requestsPerUser; j++ {
				resp, err := client.Get(fmt.Sprintf("%s/userCurrency/%d", baseURL, userID))
				mu.Lock()
				if err != nil || resp.StatusCode != http.StatusOK {
					failCount++
				} else {
					successCount++
				}
				mu.Unlock()

				if resp != nil {
					resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := concurrentUsers * requestsPerUser
	successRate := float64(successCount) / float64(totalRequests) * 100
	rps := float64(totalRequests) / duration.Seconds()

	fmt.Printf("Load Test Results:\n")
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Success Rate: %.2f%%\n", successRate)
	fmt.Printf("Requests/Second: %.2f\n", rps)
	fmt.Printf("Total Duration: %v\n", duration)
	fmt.Printf("Failed Requests: %d\n", failCount)
}