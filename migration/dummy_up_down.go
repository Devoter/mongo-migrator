package migration

import "go.mongodb.org/mongo-driver/mongo"

// DummyUpDown is a dummy migration function.
func DummyUpDown(db *mongo.Database) error {
	return nil
}
