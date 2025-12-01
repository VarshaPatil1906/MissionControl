package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	amqp "github.com/streadway/amqp"
)

type Mission struct {
	MissionID     string `json:"mission_id"`
	Payload       string `json:"payload"`
	Status        string `json:"status"`
	TargetSoldier string `json:"target_soldier"`
	Token         string `json:"token,omitempty"`
}

var (
	amqpURI     = "amqp://guest:guest@rabbitmq:5672/"
	statusQueue string
	commanderURL string
	tokenSecret  = "super_secret"

	token       string
	tokenExpiry time.Time
	mu          sync.Mutex
	wg          sync.WaitGroup

	soldierName   string
	commanderName string
)

func main() {
	rand.Seed(time.Now().UnixNano())

	soldierName = os.Getenv("SOLDIER_NAME")
	if soldierName == "" {
		log.Fatal("SOLDIER_NAME env var is required (e.g. soldier1)")
	}

	commanderName = os.Getenv("COMMANDER_NAME")
	if commanderName == "" {
		commanderName = "commander1"
	}

	ordersQueue := "orders_" + commanderName + "_" + soldierName
	statusQueue = "status_" + commanderName
	commanderURL = "http://" + commanderName + ":8080/auth/refresh_token"

	log.Printf("Starting soldier worker %s for %s, listening on %s, status queue %s",
		soldierName, commanderName, ordersQueue, statusQueue)

	go tokenRefresher()

	var (
		conn *amqp.Connection
		err  error
	)
	for {
		conn, err = amqp.Dial(amqpURI)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ: %v. Retrying in 3s...", err)
		time.Sleep(3 * time.Second)
	}
	defer conn.Close()
	log.Println("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	if _, err = ch.QueueDeclare(ordersQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("Queue Declare failed for %s: %v", ordersQueue, err)
	}
	if _, err = ch.QueueDeclare(statusQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("Queue Declare failed for statusQueue %s: %v", statusQueue, err)
	}

	msgs, err := ch.Consume(ordersQueue, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	workerPool := make(chan struct{}, 2)

	for msg := range msgs {
		var mission Mission
		if err := json.Unmarshal(msg.Body, &mission); err != nil {
			log.Printf("Failed to unmarshal mission: %v", err)
			continue
		}

		log.Printf("[%s/%s] Received mission %s (target_soldier=%s)",
			commanderName, soldierName, mission.MissionID, mission.TargetSoldier)

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
	publishStatus(ch, mission.MissionID, "IN_PROGRESS", mission.TargetSoldier)

	delay := 5 + rand.Intn(11)
	time.Sleep(time.Duration(delay) * time.Second)

	outcome := "COMPLETED"
	if rand.Float32() > 0.9 {
		outcome = "FAILED"
	}

	publishStatus(ch, mission.MissionID, outcome, mission.TargetSoldier)
}

func publishStatus(ch *amqp.Channel, missionID, status, targetSoldier string) {
	mu.Lock()
	t := token
	mu.Unlock()

	update := Mission{
		MissionID:     missionID,
		Status:        status,
		TargetSoldier: targetSoldier,
		Token:         t,
	}

	body, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal status update: %v", err)
		return
	}

	if err = ch.Publish("", statusQueue, false, false, amqp.Publishing{Body: body}); err != nil {
		log.Printf("Publish status failed: %v", err)
		return
	}

	log.Printf("[%s/%s] Mission %s status: %s by %s (Token: %s)",
		commanderName, soldierName, missionID, status, targetSoldier, t)
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
	req.Header.Set("X-SOLDIER", soldierName)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Token refresh HTTP error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Token refresh failed with status %d", resp.StatusCode)
		return
	}

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("Token refresh decode error: %v", err)
		return
	}

	if data.Token == "" {
		log.Printf("Token refresh returned empty token")
		return
	}

	mu.Lock()
	token = data.Token
	tokenExpiry = time.Now().Add(30 * time.Second)
	mu.Unlock()

	log.Printf("[%s/%s] Rotated token: %s (expires 30s)", commanderName, soldierName, data.Token)
}
