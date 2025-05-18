package main

import (
	"log"
	"mychat-room/database"
	"mychat-room/handlers"
	"mychat-room/middleware"
	"mychat-room/utils"
	"net/http"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env not found, continue with system env")
	}
	utils.InitRedis()
	database.InitMongo()

	http.Handle("/rooms", middleware.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetRoomsHandler(w, r)
		case http.MethodPost:
			// ใช้ middleware ตรวจ role admin
			middleware.RequireAdmin(http.HandlerFunc(handlers.CreateRoomHandler)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	http.Handle("/rooms/", middleware.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		log.Println("📡 Routed:", path)
		if strings.HasSuffix(path, "/join") && r.Method == http.MethodPost {
			middleware.JWTAuthMiddleware(http.HandlerFunc(handlers.JoinRoomHandler)).ServeHTTP(w, r)
			return
		}

		http.Error(w, "Not Found", http.StatusNotFound)
	})))

	log.Println("Room service running on :5000")
	log.Fatal(http.ListenAndServe(":5000", nil))
}
