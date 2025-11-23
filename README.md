# Mission Control Project

## Scope and Objectives
This project implements a secure, asynchronous command and control system for military operations using Go, RabbitMQ, and Docker. The Commander's Camp issues orders, and Soldier Workers execute them, reporting status back through a central hub.

## Design Rationale
- **GoLang:** Chosen for its concurrency, simplicity, and containerization support.
- **RabbitMQ:** Used for reliable, asynchronous message passing.
- **Token Rotation:** Short-lived tokens ensure secure identity management.

## API Documentation
- `POST /missions`: Submit a mission. Returns `mission_id`.
- `GET /missions/{mission_id}`: Get mission status.

## AI Usage Policy
AI tools were used to generate code templates, Dockerfiles, and orchestration scripts. Prompts included:
- "Go RabbitMQ example for producer/consumer"
- "Go REST API with Gin"
- "Dockerfile for Go service"

## Setup Instructions
1. Clone the repository.
2. Run `docker-compose up --build`.
3. Use `test_missions.sh` to test the system.

## How to Run Tests
Execute `./test_missions.sh` to submit and track missions.

## Architecture diagram
![alt text](image-3.png)

## Flowchart - Mission Control
![alt text](image-1.png)

## Worker Scalability and Horizontal Scaling
This project is designed for scalable, parallel processing using as many Soldier Worker containers as needed. By default, docker-compose.yml does not specify a fixed number of workers.
You can add or remove worker containers dynamically according to your workload or available resources.

If you want to run multiple workers in parallel, use the following command:

docker compose up --scale soldier_worker=N
Where N is the number of worker containers you want to run at once (for example, --scale soldier_worker=5).

There is no upper limit imposed by the configuration—you are free to scale horizontally for any use case.
If you do not specify --scale, Docker Compose will just start one worker container by default.

All workers automatically connect to RabbitMQ, share the task queue, and independently process missions—enhancing reliability and system throughput.