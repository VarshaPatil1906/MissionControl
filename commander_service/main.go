package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	amqp "github.com/streadway/amqp"
)

type Mission struct {
	MissionID     string `json:"mission_id"`
	Payload       string `json:"payload"`
	Status        string `json:"status"`
	TargetSoldier string `json:"target_soldier"`

	AssignedSoldier string `json:"assigned_soldier"` // NEW
	CommanderName   string `json:"commander_name"`   // NEW
}

var (
	missionStatus = make(map[string]*Mission)
	mu            sync.Mutex

	amqpURI     = "amqp://guest:guest@rabbitmq:5672/"
	statusQueue string
	tokenSecret = "super_secret"

	statusStore = NewMissionStatusStore()

	// store only SHA-256 hash of latest token per soldier
	soldierTokens   = make(map[string]string)
	soldierTokensMu sync.Mutex

	commanderName string
)

// status message coming back from soldiers
type StatusUpdate struct {
	MissionID     string `json:"mission_id"`
	Status        string `json:"status"`
	TargetSoldier string `json:"target_soldier"`
	Token         string `json:"token"` // plain token sent by soldier
}

// generate secure random token + its SHA-256 hash
func newToken() (plain string, hash string, err error) {
	b := make([]byte, 32) // 256 bits
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(b)

	h := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(h[:])
	return plain, hash, nil
}

func main() {
	log.Println("Commander Service starting...")

	commanderName = os.Getenv("COMMANDER_NAME")
	if commanderName == "" {
		commanderName = "commander1"
	}
	statusQueue = "status_" + commanderName
	log.Printf("Commander name: %s, status queue: %s", commanderName, statusQueue)

	// Connect to RabbitMQ
	var (
		conn *amqp.Connection
		err  error
	)
	for {
		conn, err = amqp.Dial(amqpURI)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ connection failed: %v. Retrying in 3s...", err)
		time.Sleep(3 * time.Second)
	}
	defer conn.Close()
	log.Println("Connected to RabbitMQ!")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("RabbitMQ channel open failed: %v", err)
	}
	defer ch.Close()
	log.Println("RabbitMQ channel opened.")

	if _, err = ch.QueueDeclare(statusQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("Queue Declare failed for statusQueue %s: %v", statusQueue, err)
	}
	log.Println("RabbitMQ status queue declared:", statusQueue)

	go consumeStatusUpdates(ch)
	log.Println("Background status update consumer started.")

	// Gin router
	router := gin.Default()
	router.Use(cors.Default())
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Next()
	})

	// POST /missions
	router.POST("/missions", func(c *gin.Context) {
		var req Mission
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		if req.TargetSoldier == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_soldier is required"})
			return
		}

		ordersQueue := fmt.Sprintf("orders_%s_%s", commanderName, req.TargetSoldier)
		if _, err := ch.QueueDeclare(ordersQueue, true, false, false, false, nil); err != nil {
			log.Printf("Queue Declare failed for %s: %v", ordersQueue, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Queue declare failed"})
			return
		}

		req.MissionID = uuid.NewString()
		req.Status = "QUEUED"

		// NEW: fill extra fields
		req.AssignedSoldier = req.TargetSoldier
		req.CommanderName = commanderName

		body, err := json.Marshal(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Marshal failed"})
			return
		}

		if err = ch.Publish("", ordersQueue, false, false, amqp.Publishing{Body: body}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Publish failed"})
			return
		}

		mu.Lock()
		missionStatus[req.MissionID] = &req
		mu.Unlock()

		statusStore.AddEventWithSoldier(req.MissionID, StatusQueued, req.TargetSoldier, "Mission received and queued")

		log.Printf("[%s] Mission submitted: %s to %s (%s)", commanderName, req.MissionID, req.TargetSoldier, ordersQueue)
		c.JSON(http.StatusAccepted, gin.H{"mission_id": req.MissionID})
	})

	// GET /missions/:id
	router.GET("/missions/:id", func(c *gin.Context) {
		id := c.Param("id")

		mu.Lock()
		mission, exists := missionStatus[id]
		mu.Unlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mission not found"})
			return
		}
		c.JSON(http.StatusOK, mission)
	})

	// GET /missions/:id/history
	router.GET("/missions/:id/history", func(c *gin.Context) {
		id := c.Param("id")

		history := statusStore.GetHistory(id)
		if len(history) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mission not found or no history"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mission_id": id,
			"events":     history,
		})
	})

	// GET /missions
	router.GET("/missions", func(c *gin.Context) {
		mu.Lock()
		allMissions := make([]Mission, 0, len(missionStatus))
		for _, v := range missionStatus {
			allMissions = append(allMissions, *v)
		}
		mu.Unlock()
		c.JSON(http.StatusOK, allMissions)
	})

	// POST /auth/refresh_token – secure token rotation
	router.POST("/auth/refresh_token", func(c *gin.Context) {
		secret := c.GetHeader("X-SECRET")
		if secret != tokenSecret {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		soldier := c.GetHeader("X-SOLDIER")
		if soldier == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-SOLDIER header required"})
			return
		}

		plain, hash, err := newToken()
		if err != nil {
			log.Printf("Failed to generate token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		// store only hash
		soldierTokensMu.Lock()
		soldierTokens[soldier] = hash
		soldierTokensMu.Unlock()

		log.Printf("[%s] TOKEN ROTATED for %s (hash=%s)", commanderName, soldier, hash)

		// send plain token back to soldier
		c.JSON(http.StatusOK, gin.H{"token": plain})
	})

	// GET /soldiers/:name/token
	router.GET("/soldiers/:name/token", func(c *gin.Context) {
		name := c.Param("name")

		soldierTokensMu.Lock()
		hash, ok := soldierTokens[name]
		soldierTokensMu.Unlock()

		if !ok || hash == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "No token for this soldier yet"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"soldier": name,
			"token":   hash,
		})
	})

	addr := ":8080"
	log.Println("Commander Service listening on", addr, "as", commanderName)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Gin server failed: %v", err)
	}
}

// consumeStatusUpdates verifies token hash and updates state/history
func consumeStatusUpdates(ch *amqp.Channel) {
	msgs, err := ch.Consume(statusQueue, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	for msg := range msgs {
		var upd StatusUpdate
		if err := json.Unmarshal(msg.Body, &upd); err != nil {
			log.Printf("Status update unmarshal error: %v", err)
			continue
		}

		// verify token if we have a hash stored for this soldier
		if upd.TargetSoldier != "" && upd.Token != "" {
			h := sha256.Sum256([]byte(upd.Token))
			gotHash := hex.EncodeToString(h[:])

			soldierTokensMu.Lock()
			expectedHash := soldierTokens[upd.TargetSoldier]
			soldierTokensMu.Unlock()

			if expectedHash == "" || expectedHash != gotHash {
				log.Printf("[%s] INVALID TOKEN for soldier %s on mission %s",
					commanderName, upd.TargetSoldier, upd.MissionID)
				continue
			}
		}

		// update latest snapshot
		mu.Lock()
		if mission, ok := missionStatus[upd.MissionID]; ok {
			mission.Status = upd.Status
			if upd.TargetSoldier != "" {
				mission.TargetSoldier = upd.TargetSoldier
				mission.AssignedSoldier = upd.TargetSoldier // keep in sync
			}
		}
		mu.Unlock()

		var st MissionStatus
		switch upd.Status {
		case "QUEUED":
			st = StatusQueued
		case "IN_PROGRESS":
			st = StatusInProgress
		case "COMPLETED":
			st = StatusCompleted
		case "FAILED":
			st = StatusFailed
		default:
			log.Printf("Unknown status %q for mission %s", upd.Status, upd.MissionID)
			continue
		}

		statusStore.AddEventWithSoldier(upd.MissionID, st, upd.TargetSoldier, "")
		log.Printf("[%s] Updated Mission %s to %s by %s", commanderName, upd.MissionID, upd.Status, upd.TargetSoldier)
	}
}
