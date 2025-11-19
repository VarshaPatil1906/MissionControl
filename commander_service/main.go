package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"

    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/streadway/amqp"
)

type Mission struct {
    MissionID string `json:"mission_id"`
    Payload   string `json:"payload"`
    Status    string `json:"status"`
}

var (
    missionStatus = make(map[string]*Mission)
    mu            sync.Mutex
    amqpURI       = "amqp://guest:guest@localhost:5672/"
    ordersQueue   = "orders_queue"
    statusQueue   = "status_queue"
    tokenSecret   = "super_secret"
)

func main() {
    log.Println("Commander Service starting...")

    // Connect to RabbitMQ
    conn, err := amqp.Dial(amqpURI)
    if err != nil {
        log.Fatalf("RabbitMQ connection failed: %v", err)
    }
    defer conn.Close()
    log.Println("Connected to RabbitMQ!")

    // Open channel
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("RabbitMQ channel open failed: %v", err)
    }
    defer ch.Close()
    log.Println("RabbitMQ channel opened.")

    // Declare queues
    _, err = ch.QueueDeclare(ordersQueue, true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Queue Declare failed for ordersQueue: %v", err)
    }
    _, err = ch.QueueDeclare(statusQueue, true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Queue Declare failed for statusQueue: %v", err)
    }
    log.Println("RabbitMQ queues declared.")

    // Background goroutine to consume mission status updates
    go consumeStatusUpdates(ch)
    log.Println("Background status update consumer started.")

    // Setup Gin router with CORS
    router := gin.Default()
    router.Use(cors.Default())
    router.Use(func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Next()
    })

    // POST /missions: issue mission
    router.POST("/missions", func(c *gin.Context) {
        var req Mission
        if err := c.BindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
            return
        }
        req.MissionID = uuid.NewString()
        req.Status = "QUEUED"

        body, err := json.Marshal(req)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Marshal failed"})
            return
        }

        err = ch.Publish("", ordersQueue, false, false, amqp.Publishing{Body: body})
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Publish failed"})
            return
        }

        mu.Lock()
        missionStatus[req.MissionID] = &req
        mu.Unlock()

        log.Printf("Mission submitted: %s", req.MissionID)
        c.JSON(http.StatusAccepted, gin.H{"mission_id": req.MissionID})
    })

    // GET /missions/:id: get mission status by ID
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

    // GET /missions: get all missions (for dashboard list)
    router.GET("/missions", func(c *gin.Context) {
        mu.Lock()
        allMissions := make([]Mission, 0, len(missionStatus))
        for _, v := range missionStatus {
            allMissions = append(allMissions, *v)
        }
        mu.Unlock()
        c.JSON(http.StatusOK, allMissions)
    })

    // POST /auth/refresh_token: token rotation endpoint
    router.POST("/auth/refresh_token", func(c *gin.Context) {
        secret := c.GetHeader("X-SECRET")
        if secret != tokenSecret {
            c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
            return
        }

        newToken := uuid.NewString()
        log.Printf("TOKEN ROTATED: %s", newToken)
        c.JSON(http.StatusOK, gin.H{"token": newToken})
    })

    log.Println("Commander Service listening on :8080")
    if err := router.Run(":8080"); err != nil {
        log.Fatalf("Gin server failed: %v", err)
    }
}

func consumeStatusUpdates(ch *amqp.Channel) {
    msgs, err := ch.Consume(statusQueue, "", true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Failed to register consumer: %v", err)
    }

    for msg := range msgs {
        var upd Mission
        err := json.Unmarshal(msg.Body, &upd)
        if err != nil {
            log.Printf("Status update unmarshal error: %v", err)
            continue
        }

        mu.Lock()
        if mission, ok := missionStatus[upd.MissionID]; ok {
            mission.Status = upd.Status
        }
        mu.Unlock()

        log.Printf("Updated Mission %s to %s", upd.MissionID, upd.Status)
    }
}
