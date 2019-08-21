package migration

import "go.mongodb.org/mongo-driver/mongo"

// ApplyFunc declares func type for migration functions
type ApplyFunc func(db *mongo.Database) error

// Migration declares a migration data structure.
type Migration struct {
	Version int64     `json:"version" bson:"version"`
	Name    string    `json:"name" bson:"name"`
	Up      ApplyFunc `json:"-" bson:"-"`
	Down    ApplyFunc `json:"-" bson:"-"`
	Stored  bool      `json:"-" bson:"-"`
}

// Less returns `true` if an argument is more than current.
func (mig *Migration) Less(migration *Migration) bool {
	return CompareMigrations(mig, migration)
}

// Eq returns `true` if migrations version are equal.
func (mig *Migration) Eq(migration *Migration) bool {
	return mig.Version == mig.Version
}

// Migrations type declares a slice-type of `Migration` with an implementation of `sort.Sort` interface.
type Migrations []Migration

func (ms Migrations) Len() int {
	return len(ms)
}

func (ms Migrations) Swap(i int, j int) {
	ms[i], ms[j] = ms[j], ms[i]
}

func (ms Migrations) Less(i int, j int) bool {
	return CompareMigrations(&ms[i], &ms[j])
}

// CompareMigrations compares two migrations and returns `true` if `left` migration is less.
func CompareMigrations(left *Migration, right *Migration) bool {
	return left.Version < right.Version
}
