package impls

import (
	"context"
	"fmt"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoCustomerRepository struct {
	collection *mongo.Collection
}

func NewMongoCustomerRepository(col *mongo.Collection) *MongoCustomerRepository {
	return &MongoCustomerRepository{collection: col}
}

func (r *MongoCustomerRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetName("idx_customers_email_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_customers_created_at"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create customer indexes: %w", err)
	}
	return nil
}

func (r *MongoCustomerRepository) Insert(ctx context.Context, customer models.Customer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := r.collection.InsertOne(ctx, customer)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoCustomerRepository) FindByEmail(ctx context.Context, email string) (models.Customer, error) {
	return r.findOne(ctx, bson.M{"email": email})
}

func (r *MongoCustomerRepository) FindByID(ctx context.Context, id string) (models.Customer, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.Customer{}, mongo.ErrNoDocuments
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

func (r *MongoCustomerRepository) findOne(ctx context.Context, filter bson.M) (models.Customer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var customer models.Customer
	if err := r.collection.FindOne(ctx, filter).Decode(&customer); err != nil {
		return models.Customer{}, err
	}
	return customer, nil
}

type MongoCustomerSessionRepository struct {
	collection *mongo.Collection
}

func NewMongoCustomerSessionRepository(col *mongo.Collection) *MongoCustomerSessionRepository {
	return &MongoCustomerSessionRepository{collection: col}
}

func (r *MongoCustomerSessionRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tokenHash", Value: 1}},
			Options: options.Index().SetName("idx_customer_sessions_token_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetName("idx_customer_sessions_expiry").SetExpireAfterSeconds(0),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create customer session indexes: %w", err)
	}
	return nil
}

func (r *MongoCustomerSessionRepository) Insert(ctx context.Context, session models.CustomerSession) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.InsertOne(ctx, session)
	return err
}

func (r *MongoCustomerSessionRepository) FindByTokenHash(ctx context.Context, tokenHash string) (models.CustomerSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var session models.CustomerSession
	if err := r.collection.FindOne(ctx, bson.M{"tokenHash": tokenHash}).Decode(&session); err != nil {
		return models.CustomerSession{}, err
	}
	return session, nil
}

func (r *MongoCustomerSessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.DeleteOne(ctx, bson.M{"tokenHash": tokenHash})
	return err
}

type MongoEventPlanRepository struct {
	collection *mongo.Collection
}

func NewMongoEventPlanRepository(col *mongo.Collection) *MongoEventPlanRepository {
	return &MongoEventPlanRepository{collection: col}
}

func (r *MongoEventPlanRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_event_plans_code_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "active", Value: 1}, {Key: "sortOrder", Value: 1}},
			Options: options.Index().SetName("idx_event_plans_active_order"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create event plan indexes: %w", err)
	}
	return nil
}

func (r *MongoEventPlanRepository) SeedDefaults(ctx context.Context, plans []models.EventPlan) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for _, plan := range plans {
		_, err := r.collection.UpdateOne(ctx,
			bson.M{"code": plan.Code},
			bson.M{"$setOnInsert": plan},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MongoEventPlanRepository) List(ctx context.Context) ([]models.EventPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "price", Value: 1}})
	cur, err := r.collection.Find(ctx, bson.M{"active": true}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var plans []models.EventPlan
	for cur.Next(ctx) {
		var plan models.EventPlan
		if err := cur.Decode(&plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, cur.Err()
}

func (r *MongoEventPlanRepository) FindByCode(ctx context.Context, code string) (models.EventPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var plan models.EventPlan
	if err := r.collection.FindOne(ctx, bson.M{"code": code}).Decode(&plan); err != nil {
		return models.EventPlan{}, err
	}
	return plan, nil
}

func (r *MongoEventPlanRepository) Update(ctx context.Context, plan models.EventPlan) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := r.collection.UpdateOne(ctx, bson.M{"code": plan.Code}, bson.M{"$set": plan})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
