package handlers

import (
	"context"
	"encoding/json"
	"log"
	"mychat-room/contextkey"
	"mychat-room/database"
	"mychat-room/models"
	"mychat-room/utils"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GET /rooms
func GetRoomsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := database.RoomCollection.Find(ctx, bson.M{}) //  ใช้ collection จาก database package
	if err != nil {
		http.Error(w, "Failed to fetch rooms", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var rooms []models.Room
	if err := cursor.All(ctx, &rooms); err != nil {
		http.Error(w, "Failed to decode rooms", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func CreateRoomHandler(w http.ResponseWriter, r *http.Request) {
	tokenCookie, err := r.Cookie("token")
	if err != nil || tokenCookie.Value == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	claims, err := utils.ValidateToken(tokenCookie.Value)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	if claims.Role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	var req models.Room
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || (req.Type != "public" && req.Type != "private") {
		http.Error(w, "Invalid room data", http.StatusBadRequest)
		return
	}

	count, _ := database.RoomCollection.CountDocuments(context.TODO(), bson.M{"name": req.Name})
	if count > 0 {
		http.Error(w, "Room name already exists", http.StatusConflict)
		return
	}

	creatorID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
		return
	}

	var creator models.User
	err = database.UserCollection.FindOne(context.TODO(), bson.M{"_id": creatorID}).Decode(&creator)
	if err != nil {
		http.Error(w, "Failed to load creator user", http.StatusInternalServerError)
		return
	}

	safeCreator := creator.ToSafeUser()

	req.ID = primitive.NewObjectID()
	req.CreatedAt = time.Now()
	req.Members = []models.SafeUser{safeCreator}

	_, err = database.RoomCollection.InsertOne(context.TODO(), req)
	if err != nil {
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func JoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("📥 JoinRoomHandler called")

	roomID := strings.TrimPrefix(r.URL.Path, "/rooms/")
	roomID = strings.TrimSuffix(roomID, "/join")
	log.Println("🔍 roomID =", roomID)

	userID, ok := r.Context().Value(contextkey.UserID).(string)
	if !ok || userID == "" {
		log.Println("❌ userID missing from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Println("👤 userID =", userID)

	roomObjID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		log.Println("❌ Invalid room ID:", err)
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userObjID := models.StringToObjectID(userID)

	var user models.User
	err = database.UserCollection.FindOne(context.TODO(), bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		log.Println("❌ User not found:", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Println("✅ User found:", user.Email)

	safeUser := user.ToSafeUser()

	filter := bson.M{"_id": roomObjID}
	update := bson.M{"$addToSet": bson.M{"members": safeUser}}

	res, err := database.RoomCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Println("❌ DB update error:", err)
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}

	if res.MatchedCount == 0 {
		log.Println("❌ Room not found:", roomID)
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	log.Println("✅ User joined room:", roomID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Joined room"})
}

func GetRoomMessagesHandler(w http.ResponseWriter, r *http.Request) {
	roomIDStr := strings.TrimPrefix(r.URL.Path, "/rooms/")
	roomIDStr = strings.TrimSuffix(roomIDStr, "/messages")
	roomID, err := primitive.ObjectIDFromHex(roomIDStr)
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	filter := bson.M{"room_id": roomID}
	log.Println("🧾 MongoDB filter:", filter)

	cursor, err := database.MessageCollection.Find(context.TODO(), filter)
	if err != nil {
		log.Println("❌ Failed to fetch messages:", err)
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	var messages []models.Message
	if err := cursor.All(context.TODO(), &messages); err != nil {
		log.Println("❌ Failed to decode messages:", err)
		http.Error(w, "Failed to decode messages", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Messages fetched: %d\n", len(messages))
	for _, msg := range messages {
		log.Printf("📨 [%s] %s\n", msg.SenderID.Hex(), msg.Content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
