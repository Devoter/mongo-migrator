package migrator

import (
	"context"
	"sort"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Devoter/mongo-migrator/migration"
)

// Migrator declares MongoDB migrations manager.
type Migrator struct {
	client     *mongo.Client
	migrations []migration.Migration
}

// NewMigrator returns a new instance of `Migrator`.
func NewMigrator(client *mongo.Client, migrations []migration.Migration) *Migrator {
	all := append(migrations, migration.Migration{Name: "-", Up: migration.DummyUpDown, Down: migration.DummyUpDown})
	sort.Sort(migration.Migrations(all))

	return &Migrator{
		client:     client,
		migrations: all,
	}
}

// Run interprets commands.
func (m *Migrator) Run(db string, args ...string) (oldVersion int64, newVersion int64, err error) {
	if len(args) == 0 {
		err = ErrorCommandRequired
		return
	}

	base := m.client.Database(db)

	switch args[0] {
	case "init":
		return m.Init(base)
	case "up":
		var target int64

		target, err = m.parseVersion(false, args[1:]...)
		if err != nil {
			return
		}

		return m.Up(base, target)
	case "down":
		return m.Down(base)
	case "reset":
		return m.Reset(base)
	case "version":
		return m.Version(base)
	case "set_version":
		var target int64

		target, err = m.parseVersion(true, args[1:]...)
		if err != nil {
			return
		}

		return m.SetVersion(base, target)
	default:
		err = ErrorUnexpectedCommand
		return
	}
}

// Init creates `migrations` collection if it does not exist and records the initial zero-migration.
func (m *Migrator) Init(db *mongo.Database) (oldVersion int64, newVersion int64, err error) {
	migr := &migration.Migration{Name: "-"}
	var mig migration.Migration
	result := db.Collection("migrations").FindOne(context.TODO(), bson.D{{"version", 0}})
	if err = result.Err(); err != nil {
		if err != mongo.ErrNoDocuments {
			return
		}
	} else if result.Decode(&mig) == nil {
		err = ErrorMigrationsCollectionAlreadyExists
		return
	}

	_, err = db.Collection("migrations", migration.MajorityOpts()).InsertOne(context.TODO(), migr)
	return
}

// Up upgrades database revision to the target or next version.
func (m *Migrator) Up(db *mongo.Database, target int64) (oldVersion int64, newVersion int64, err error) {
	opts := options.Find()
	opts.SetSort(bson.D{{"version", 1}})

	cursor, err := db.Collection("migrations").Find(context.TODO(), bson.D{}, opts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = ErrorMigrationsAreNotInitialized
		}
		return
	}
	defer cursor.Close(context.TODO())

	history := []migration.Migration{}

	for cursor.Next(context.TODO()) {
		var mig migration.Migration

		if err = cursor.Decode(&mig); err != nil {
			return
		}

		mig.Stored = true
		history = append(history, mig)
	}

	length := len(history)
	if length > 0 {
		version := history[length-1].Version
		oldVersion = version
		newVersion = version
	}

	coll := db.Collection("migrations", migration.MajorityOpts())
	merged := m.mergeMigrations(history, m.migrations, target)

	for _, migr := range merged {
		if !migr.Stored {
			newVersion = migr.Version

			if err = migr.Up(db); err != nil {
				return
			}

			migr.Stored = true

			if _, err = coll.InsertOne(context.TODO(), &migr); err != nil {
				return
			}
		}
	}

	return
}

// Down downgrades database revision to the previous version.
func (m *Migrator) Down(db *mongo.Database) (oldVersion int64, newVersion int64, err error) {
	opts := options.FindOne()

	opts.SetSort(bson.D{{"version", -1}})

	var old migration.Migration

	result := db.Collection("migrations").FindOne(context.TODO(), bson.D{}, opts)
	if err = result.Err(); err != nil {
		if err == mongo.ErrNoDocuments {
			err = ErrorMigrationsAreNotInitialized
		}
		return
	} else if err = result.Decode(&old); err != nil {
		return
	}

	oldVersion = old.Version
	newVersion = old.Version
	coll := db.Collection("migrations", migration.MajorityOpts())

	for i := len(m.migrations) - 1; i >= 0; i-- {
		mig := m.migrations[i]

		if mig.Version == old.Version {
			if i > 0 {
				newVersion = m.migrations[i-1].Version

				if err = mig.Down(db); err != nil {
					return
				}

				_, err = coll.DeleteOne(context.TODO(), bson.D{{"version", mig.Version}})

			}

			return
		}
	}

	return
}

// Reset resets database to the zero-revision.
func (m *Migrator) Reset(db *mongo.Database) (oldVersion int64, newVersion int64, err error) {
	opts := options.Find()
	opts.SetSort(bson.D{{"version", 1}})

	cursor, err := db.Collection("migrations").Find(context.TODO(), bson.D{}, opts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = ErrorMigrationsAreNotInitialized
		}
		return
	}
	defer cursor.Close(context.TODO())

	history := []migration.Migration{}

	for cursor.Next(context.TODO()) {
		var mig migration.Migration

		if err = cursor.Decode(&mig); err != nil {
			return
		}

		mig.Stored = true
		history = append(history, mig)
	}

	length := len(history)
	if length > 0 {
		version := history[length-1].Version
		oldVersion = version
		newVersion = version
	} else {
		return
	}

	coll := db.Collection("migrations", migration.MajorityOpts())
	correlated, err := m.correlateMigrations(history, m.migrations)
	if err != nil {
		return
	}

	for i := len(correlated) - 1; i >= 0; i-- {
		migr := correlated[i]

		if i > 0 {
			newVersion = correlated[i-1].Version
		} else {
			newVersion = migr.Version
		}

		if err = migr.Down(db); err != nil {
			return
		}

		migr.Stored = true

		// don't delete zero migration
		if migr.Version > 0 {
			if _, err = coll.DeleteOne(context.TODO(), bson.D{{"version", migr.Version}}); err != nil {
				return
			}
		}
	}

	return
}

// Version returns current database revision version.
func (m *Migrator) Version(db *mongo.Database) (oldVersion int64, newVersion int64, err error) {
	opts := options.FindOne()
	opts.SetSort(bson.D{{"version", -1}})

	var mig migration.Migration

	result := db.Collection("migrations").FindOne(context.TODO(), bson.D{}, opts)
	if err = result.Err(); err != nil {
		if err == mongo.ErrNoDocuments {
			err = ErrorMigrationsAreNotInitialized
		}
		return
	} else if err = result.Decode(&mig); err != nil {
		return
	}

	oldVersion = mig.Version
	newVersion = mig.Version
	return
}

// SetVersion forces database revisiton version.
func (m *Migrator) SetVersion(db *mongo.Database, target int64) (oldVersion int64, newVersion int64, err error) {
	oldVersion, _, err = m.Version(db)
	if err != nil {
		return
	}

	index := -1
	migs := make([]interface{}, 0, len(m.migrations))

	for i, migr := range m.migrations {
		migs = append(migs, migr)
		if migr.Version == target {
			index = i
			break
		}
	}

	if index == -1 {
		err = ErrorTargetVersionNotFound
		return
	} else if oldVersion == m.migrations[index].Version {
		newVersion = oldVersion
		return
	}

	coll := db.Collection("migrations", migration.MajorityOpts())
	if err = coll.Drop(context.TODO()); err != nil {
		return
	}

	if _, err = coll.InsertMany(context.TODO(), migs); err != nil {
		return
	}

	newVersion = migs[len(migs)-1].(migration.Migration).Version
	return
}

func (m *Migrator) parseVersion(required bool, args ...string) (version int64, err error) {
	if len(args) == 0 {
		if required {
			err = ErrorVersionNumberRequired
			return
		}

		version = -1
		return
	}

	version, err = strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		err = ErrorInvalidVersionArgumentFormat
		return
	}

	return
}

// mergreMigrations returns a slice contains a sorted list of all migrations (applied and actual).
func (m *Migrator) mergeMigrations(applied, actual []migration.Migration, target int64) []migration.Migration {
	appliedLength := len(applied)
	actualLength := len(actual)
	merged := make([]migration.Migration, 0, appliedLength+actualLength)
	i := 0
	j := 0
	var max int64

	if actualLength > 0 {
		if target == -1 {
			max = actual[actualLength-1].Version + 1
		} else {
			max = target + 1
		}
	}

	for (i < appliedLength) && (j < actualLength) && (actualLength == 0 || actual[j].Version < max) {
		if applied[i].Less(&actual[j]) {
			merged = append(merged, applied[i])
			i++
		} else if actual[j].Less(&applied[j]) {
			merged = append(merged, actual[j])
			j++
		} else {
			merged = append(merged, applied[i])
			i++
			j++
		}
	}

	for i < appliedLength {
		merged = append(merged, applied[i])
		i++
	}

	for j < actualLength && (actualLength == 0 || actual[j].Version < max) {
		merged = append(merged, actual[j])
		j++
	}

	return merged
}

// CorrelateMigrations returns a list of correlated migrations.
// This method replaces stored migrations with actual migrations. If some actual migration is absent
// the method returns an error and a list which contains missing migration as the last item.
func (m *Migrator) correlateMigrations(applied, actual []migration.Migration) (correlated []migration.Migration, err error) {
	appliedLength := len(applied)
	actualLength := len(actual)
	i := 0
	j := 0
	correlated = make([]migration.Migration, 0, appliedLength)

	for (i < appliedLength) && (j < actualLength) {
		if applied[i].Less(&actual[j]) {
			correlated = append(correlated, applied[i])
			err = ErrorSomeMigrationsAreAbsent
			return
		} else if actual[j].Less(&applied[i]) {
			// skip unapplied migrations
			j++
		} else {
			correlated = append(correlated, actual[j])
			i++
			j++
		}
	}

	if i < appliedLength {
		correlated = append(correlated, applied[i])
		err = ErrorSomeMigrationsAreAbsent
	}

	return
}
