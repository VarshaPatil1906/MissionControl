package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

type Mission struct {
	MissionID string `json:"mission_id"`
	Payload   string `json:"payload"`
	Status    string `json:"status"`
	Token     string `json:"token,omitempty"`
}

var (
	amqpURI = "amqp://guest:guest@localhost:5672/"
	ordersQueue  = "orders_queue"
	statusQueue  = "status_queue"
	commanderURL = "http://localhost:8080/auth/refresh_token"
	tokenSecret  = "super_secret"

	token       string
	tokenExpiry time.Time
	mu          sync.Mutex
	wg          sync.WaitGroup
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Periodically refresh authentication token
	go tokenRefresher()

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(ordersQueue, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Queue Declare failed: %v", err)
	}

	_, err = ch.QueueDeclare(statusQueue, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Queue Declare failed: %v", err)
	}

	msgs, err := ch.Consume(ordersQueue, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	workerPool := make(chan struct{}, 8) // 8 concurrent missions max

	for msg := range msgs {
		var mission Mission
		if err := json.Unmarshal(msg.Body, &mission); err != nil {
			log.Printf("Failed to unmarshal mission: %v", err)
			continue
		}

		workerPool <- struct{}{}
		wg.Add(1)

		go func(m Mission) {
			defer wg.Done()
			executeMission(ch, m)
			<-workerPool
		}(mission)
	}

	wg.Wait()
}

func executeMission(ch *amqp.Channel, mission Mission) {
	// Publish IN_PROGRESS
	publishStatus(ch, mission.MissionID, "IN_PROGRESS")

	// Simulate mission delay 5-15 sec
	delay := 5 + rand.Intn(11)
	time.Sleep(time.Duration(delay) * time.Second)

	// Outcome with 90% success rate
	outcome := "COMPLETED"
	if rand.Float32() > 0.9 {
		outcome = "FAILED"
	}

	publishStatus(ch, mission.MissionID, outcome)
}

func publishStatus(ch *amqp.Channel, missionID, status string) {
	mu.Lock()
	t := token
	mu.Unlock()

	update := Mission{
		MissionID: missionID,
		Status:    status,
		Token:     t,
	}

	body, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal status update: %v", err)
		return
	}

	err = ch.Publish("", statusQueue, false, false, amqp.Publishing{Body: body})
	if err != nil {
		log.Printf("Publish status failed: %v", err)
		return
	}

	log.Printf("Mission %s status: %s (Token: %s)", missionID, status, t)
}

func tokenRefresher() {
	for {
		mu.Lock()
		expired := token == "" || time.Now().After(tokenExpiry)
		mu.Unlock()

		if expired {
			refreshToken()
		}
		time.Sleep(2 * time.Second)
	}
}

func refreshToken() {
	req, err := http.NewRequest("POST", commanderURL, nil)
	if err != nil {
		log.Printf("Token refresh request error: %v", err)
		return
	}
	req.Header.Set("X-SECRET", tokenSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Token refresh HTTP error: %v", err)
		return
	}
	defer resp.Body.Close()

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("Token refresh decode error: %v", err)
		return
	}

	mu.Lock()
	token = data.Token
	tokenExpiry = time.Now().Add(30 * time.Second)
	mu.Unlock()

	log.Printf("Rotated token: %s (expires 30s)", data.Token)
}
