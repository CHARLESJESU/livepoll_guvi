package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password"`
}

type Poll struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID   primitive.ObjectID `bson:"ownerId" json:"ownerId"`
	Question  string             `bson:"question" json:"question"`
	Options   []string           `bson:"options" json:"options"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type Counts struct {
	Options []int `json:"options"`
	Total   int   `json:"total"`
}

type App struct {
	users  *mongo.Collection
	polls  *mongo.Collection
	votes  *mongo.Collection
	redis  *redis.Client
	secret []byte
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createPollRequest struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

type voteRequest struct {
	Option int    `json:"option"`
	Voter  string `json:"voter"`
}

func main() {
	ctx := context.Background()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(env("MONGO_URL", "mongodb://localhost:27017")))
	if err != nil {
		panic(err)
	}
	if err = mongoClient.Ping(ctx, nil); err != nil {
		panic(err)
	}
	db := mongoClient.Database(env("MONGO_DB", "livepoll"))
	redisOptions, err := redis.ParseURL(env("REDIS_URL", "redis://localhost:6379"))
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewClient(redisOptions)
	if err = redisClient.Ping(ctx).Err(); err != nil {
		panic(err)
	}

	secret := env("SESSION_SECRET", "")
	if secret == "" || secret == "change-me-in-production" || secret == "replace-this-secret" {
		panic("SESSION_SECRET must be set to a non-default value")
	}
	app := &App{users: db.Collection("users"), polls: db.Collection("polls"), votes: db.Collection("votes"), redis: redisClient, secret: []byte(secret)}
	_, _ = app.users.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)})
	_, _ = app.votes.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "voter", Value: 1}}, Options: options.Index().SetUnique(true)})

	router := gin.Default()
	router.Use(cors.New(cors.Config{AllowOrigins: []string{env("FRONTEND_URL", "http://localhost:5173")}, AllowHeaders: []string{"Origin", "Content-Type", "Authorization"}, AllowMethods: []string{"GET", "POST", "OPTIONS"}}))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.POST("/api/auth/signup", app.signup)
	router.POST("/api/auth/login", app.login)
	router.POST("/api/polls", app.requireAuth, app.createPoll)
	router.GET("/api/polls/:id", app.getPoll)
	router.POST("/api/polls/:id/votes", app.vote)
	router.GET("/api/polls/:id/events", app.events)
	srv := &http.Server{
		Addr:    ":" + env("PORT", "8080"),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)
	_ = mongoClient.Disconnect(shutdownCtx)
	_ = redisClient.Close()
}

func (a *App) signup(c *gin.Context) {
	var req authRequest
	if !decodeJSON(c, &req) || !validCredentials(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email and password of at least 8 characters are required"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := User{ID: primitive.NewObjectID(), Email: strings.ToLower(strings.TrimSpace(req.Email)), Password: string(hash)}
	_, err := a.users.InsertOne(c, user)
	if mongo.IsDuplicateKeyError(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "email is already registered"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": a.token(user.ID), "email": user.Email})
}

func (a *App) login(c *gin.Context) {
	var req authRequest
	if !decodeJSON(c, &req) {
		return
	}
	var user User
	err := a.users.FindOne(c, bson.M{"email": strings.ToLower(strings.TrimSpace(req.Email))}).Decode(&user)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": a.token(user.ID), "email": user.Email})
}

func (a *App) requireAuth(c *gin.Context) {
	id, ok := a.parseToken(c.GetHeader("Authorization"))
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign in required"})
		return
	}
	c.Set("userID", id)
	c.Next()
}

func (a *App) createPoll(c *gin.Context) {
	var req createPollRequest
	if !decodeJSON(c, &req) || len(strings.TrimSpace(req.Question)) < 5 || len(req.Options) < 2 || len(req.Options) > 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question needs 5+ characters and there must be 2-6 options"})
		return
	}
	for i := range req.Options {
		req.Options[i] = strings.TrimSpace(req.Options[i])
		if len(req.Options[i]) < 1 || len(req.Options[i]) > 80 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "options must be 1-80 characters"})
			return
		}
	}
	poll := Poll{ID: primitive.NewObjectID(), OwnerID: c.MustGet("userID").(primitive.ObjectID), Question: strings.TrimSpace(req.Question), Options: req.Options, CreatedAt: time.Now().UTC()}
	if _, err := a.polls.InsertOne(c, poll); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save poll"})
		return
	}
	if err := a.setCounts(c, poll.ID, len(poll.Options), make([]int, len(poll.Options))); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not initialize poll"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"poll": poll, "counts": Counts{Options: make([]int, len(poll.Options))}})
}

func (a *App) getPoll(c *gin.Context) {
	poll, ok := a.findPoll(c)
	if !ok {
		return
	}
	counts, err := a.getCounts(c, poll.ID, len(poll.Options))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load results"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"poll": poll, "counts": counts})
}

func (a *App) vote(c *gin.Context) {
	poll, ok := a.findPoll(c)
	if !ok {
		return
	}
	var req voteRequest
	if !decodeJSON(c, &req) || req.Option < 0 || req.Option >= len(poll.Options) || len(req.Voter) < 16 || len(req.Voter) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vote"})
		return
	}
	_, err := a.votes.InsertOne(c, bson.M{"pollId": poll.ID, "voter": req.Voter, "option": req.Option, "createdAt": time.Now().UTC()})
	if mongo.IsDuplicateKeyError(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "you have already voted"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record vote"})
		return
	}
	key := fmt.Sprintf("poll:%s:counts", poll.ID.Hex())
	values, _ := a.redis.HIncrBy(c, key, fmt.Sprint(req.Option), 1).Result()
	counts, _ := a.getCounts(c, poll.ID, len(poll.Options))
	payload, _ := json.Marshal(counts)
	_ = a.redis.Publish(c, "poll:"+poll.ID.Hex(), payload).Err()
	c.JSON(http.StatusCreated, gin.H{"counts": counts, "updated": values})
}

func (a *App) events(c *gin.Context) {
	poll, ok := a.findPoll(c)
	if !ok {
		return
	}
	sub := a.redis.Subscribe(c, "poll:"+poll.ID.Hex())
	defer sub.Close()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	initial, err := a.getCounts(c, poll.ID, len(poll.Options))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	writeEvent(c, initial)
	for {
		msg, err := sub.ReceiveMessage(c)
		if err != nil {
			return
		}
		var counts Counts
		if json.Unmarshal([]byte(msg.Payload), &counts) == nil {
			writeEvent(c, counts)
		}
	}
}

func (a *App) findPoll(c *gin.Context) (Poll, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return Poll{}, false
	}
	var poll Poll
	if err = a.polls.FindOne(c, bson.M{"_id": id}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return Poll{}, false
	}
	return poll, true
}

func (a *App) getCounts(ctx context.Context, id primitive.ObjectID, optionCount int) (Counts, error) {
	key := "poll:" + id.Hex() + ":counts"
	values, err := a.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return Counts{}, err
	}
	if len(values) < optionCount {
		if err := a.rebuildCounts(ctx, id, optionCount); err != nil {
			return Counts{}, err
		}
		values, err = a.redis.HGetAll(ctx, key).Result()
		if err != nil {
			return Counts{}, err
		}
	}
	counts := Counts{Options: make([]int, optionCount)}
	for i := range counts.Options {
		fmt.Sscanf(values[fmt.Sprint(i)], "%d", &counts.Options[i])
		counts.Total += counts.Options[i]
	}
	return counts, nil
}

func (a *App) rebuildCounts(ctx context.Context, id primitive.ObjectID, optionCount int) error {
	cursor, err := a.votes.Aggregate(ctx, mongo.Pipeline{
		{{"$match", bson.D{{"pollId", id}}}},
		{{"$group", bson.D{{"_id", "$option"}, {"count", bson.D{{"$sum", 1}}}}}},
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	counts := make([]int, optionCount)
	var result struct {
		Option int `bson:"_id"`
		Count  int `bson:"count"`
	}
	for cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return err
		}
		if result.Option >= 0 && result.Option < optionCount {
			counts[result.Option] = result.Count
		}
	}
	if err := cursor.Err(); err != nil {
		return err
	}
	return a.setCounts(ctx, id, optionCount, counts)
}

func (a *App) setCounts(ctx context.Context, id primitive.ObjectID, n int, counts []int) error {
	key := "poll:" + id.Hex() + ":counts"
	fields := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		fields[fmt.Sprint(i)] = counts[i]
	}
	return a.redis.HSet(ctx, key, fields).Err()
}

func (a *App) token(id primitive.ObjectID) string {
	raw := id.Hex() + "." + fmt.Sprint(time.Now().Add(24*time.Hour).Unix())
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(raw))
	return base64.RawURLEncoding.EncodeToString([]byte(raw+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil))))
}

func (a *App) parseToken(header string) (primitive.ObjectID, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return primitive.NilObjectID, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		return primitive.NilObjectID, false
	}
	parts := strings.Split(string(decoded), ".")
	if len(parts) != 3 {
		return primitive.NilObjectID, false
	}
	var expiry int64
	if _, err = fmt.Sscan(parts[1], &expiry); err != nil || time.Now().Unix() > expiry {
		return primitive.NilObjectID, false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return primitive.NilObjectID, false
	}
	id, err := primitive.ObjectIDFromHex(parts[0])
	return id, err == nil
}

func decodeJSON(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return false
	}
	return true
}

func validCredentials(req authRequest) bool {
	return strings.Contains(req.Email, "@") && len(req.Password) >= 8 && len(req.Password) <= 72
}

func writeEvent(c *gin.Context, payload Counts) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

