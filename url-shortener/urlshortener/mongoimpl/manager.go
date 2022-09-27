package mongoimpl

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"time"
	"url-shortener/urlshortener"
)

const dbName = "shortUrls"
const collName = "urls"

func NewManager(mongoURL string) *manager {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		panic(err)
	}

	collection := client.Database(dbName).Collection(collName)
	ensureIndexes(ctx, collection)

	return &manager{
		urls: collection,
	}
}

func ensureIndexes(ctx context.Context, collection *mongo.Collection) {
	indexModels := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "_id", Value: bsonx.Int32(1)}},
		},
	}
	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)

	_, err := collection.Indexes().CreateMany(ctx, indexModels, opts)
	if err != nil {
		panic(fmt.Errorf("failed to ensure indexes %w", err))
	}
}

type manager struct {
	urls *mongo.Collection
}

var _ urlshortener.Manager = (*manager)(nil)

func (s *manager) CreateShortcut(ctx context.Context, url string) (string, error) {
	const attemptsCount = 5

	for attempt := 0; attempt < attemptsCount; attempt++ {
		key := urlshortener.GenerateKey()
		item := urlItem{
			Key: key,
			URL: url,
		}

		_, err := s.urls.InsertOne(ctx, item)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return "", fmt.Errorf("something went wrong - %w", urlshortener.ErrStorage)
		}

		return key, nil
	}
	return "", fmt.Errorf("too much attempts during inserting - %w", urlshortener.ErrStorage)
}

func (s *manager) ResolveShortcut(ctx context.Context, key string) (string, error) {
	var result urlItem
	err := s.urls.FindOne(ctx, bson.M{"_id": key}).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", fmt.Errorf("no document with key %v - %w", key, urlshortener.ErrNotFound)
		}
		return "", fmt.Errorf("somehting went wroing - %w", urlshortener.ErrStorage)
	}
	return result.URL, nil
}
