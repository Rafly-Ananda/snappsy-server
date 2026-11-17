package db

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	Client *mongo.Client
	Db     *mongo.Database
}

func NewMongo(host string, username string, password string, dbName string, mongoOpts string) (*MongoStorage, error) {
	uri := "mongodb://"
	if username != "" {
		uri += url.UserPassword(username, password).String() + "@"
	}
	uri += strings.TrimSuffix(host, "/") + "/" + url.PathEscape(dbName)
	if mongoOpts != "" {
		uri += "?" + strings.TrimPrefix(mongoOpts, "?")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cl, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect MongoDB: %w", err)
	}

	if err = cl.Ping(ctx, nil); err != nil {
		_ = cl.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	log.Printf("Connected to MongoDB database=%s hosts=%s", dbName, host)
	return &MongoStorage{Client: cl, Db: cl.Database(dbName)}, nil
}

func (m *MongoStorage) Ping(ctx context.Context) error {
	return m.Client.Ping(ctx, nil)
}

func (m *MongoStorage) Disconnect(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}
