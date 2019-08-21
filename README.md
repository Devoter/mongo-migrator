# mongo-migrator

Mongo migrator is a library that provides migration management operations.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Contents

1. [Installation](#installation)
2. [Usage](#usage)
    1. [Migrations package](#migrations-package)
    2. [Global slice variable example](#global-slice-variable-example)
    3. [Migrations functions example](#migrations-functions-example)
3. [Commands](#commands)
    1. [Initialization (init)](#initialization-init)
    2. [Applying (up)](#applying-up)
    3. [Roll back (down)](#roll-ack-down)
    4. [Reset (reset)](#reset-reset)
    5. [Current version (version)](#current-version-version)
    6. [Version changing (set_version)](#version-changing-set_version)
4. [Direct commands calls](#direct-commands-calls)
5. [License](#license)

## Installation

With [dep](https://github.com/golang/dep):

```sh
dep ensure -add github.com/Devoter/mongo-migrator
```

Without dep:

```sh
go get -v github.com/Devoter/mongo-migrator
```

## Usage

The following instructions are only recommendations, of course, you can use the library as like as you wish.

### Migrations package

To use the library you should create migrations functions. It is intuitive to declare `migrations` package in your project. All your migrations should be placed in one slice of type `[]migration.Migration`.

#### Global slice variable example

There is a simple way to declare the migrations slice:

```go
// migrations/migrations.go
package migrations

import "github.com/Devoter/mongo-migrator/migration"

// Migrations is a list of all available migrations.
var Migrations = []migration.Migration{}
```

#### Migrations functions example

It is recommended to put all migrations functions in separate files named like `<number>.<name>.go`. Each migration file should contain `init` function that appends current migration to the global migrations slice. Every migration must have two functions: `up` and `down` that applies or rolls back the migration respectively.

```go
// migrations/1.init.go
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migration"
)

func init() {

	Migrations = append(Migrations, migration.Migration{
		Version: 1,
		Name:    "init",
		Up:      migration.DummyUpDown, // built-in dummy migration function
		Down:    Down1,
	})
}

// Down1 applies `down` migration.
func Down1(db *mongo.Database) (err error) {
	majOpts := migration.MajorityOpts()

	collections := []string{"groups", "rules", "templates", "users"}

	for _, collection := range collections {
		if err = db.Collection(collection, majOpts).Drop(context.TODO()); err != nil {
			return
		}
	}

	return
}
```

As you can see `up` and `down` functions use an official MongoDB Go driver. It is a good practice to create idempotent migrations.

### Commands

The migrator supports six commands: `init`, `up`, `down`, `reset`, `version`, and `set_version`.

#### Initialization (init)

The current migration version must be saved in the database `migrations` collection. You don't have to create it manually, just call the `init` command of the migrator:

```go
package usage_example

import (
    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// InitializeMigrations initializes the database migrations collection.
func InitializeMigrations(client *mongo.Client, dbName string, migrations []migration.Migration) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Creating the `migrations` collection and initialing it with the zero database structure version.
    if _, _, err := migrator.Run(dbName, "init"); err != nil {
        panic(err)
    }
}
```

#### Applying (up)

It is so easy to apply all migrations:

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// UpAuto applies all unapplied migrations.
func UpAuto(client *mongo.Client, dbName string, migrations []migration.Migration) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Applying all unapplied migrations.
    old, current, err := migrator.Run(dbName, "up")
    if err != nil {
        panic(err)
    }

    // Printing old and current versions. If there are no unapplied migrations `old` and `current` are equal.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

Certainly you may want to set the targe version of the database structure. It is pretty simple: just add an additional parameter with the target version.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// UpTo applyies all unapplied migrations to the target.
func UpTo(client *mongo.Client, dbName string, migrations []migration.Migration, target string) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Applying all unapplied migrations up to the target.
    old, current, err := migrator.Run(dbName, "up", target)
    if err != nil {
        panic(err)
    }

    // Printing old and current versions. If there are no unapplied migrations `old` and `current` are equal.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

#### Roll back (down)

The `down` command removes the last migration (calls the migration `down` function). This command does not support any additional parameters. It is very similar to the `up` command.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// Down reverts the last applied migration.
func Down(client *mongo.Client, dbName string, migrations []migration.Migration) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Reverting the last applied migration.
    old, current, err := migrator.Run(dbName, "down")
    if err != nil {
        panic(err)
    }

    // Printing old and current versions. If there are no applied migrations `old` and `current` are equal.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

#### Reset (reset)

This command reverts all applied migrations and sets the database structure version to zero.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// Reset reverts all applied migrations.
func Reset(client *mongo.Client, dbName string, migrations []migration.Migration) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Reverting all applied migrations.
    old, current, err := migrator.Run(dbName, "reset")
    if err != nil {
        panic(err)
    }

    // Printing old and current versions. If there are no applied migrations `old` and `current` are equal.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

#### Current version (version)

This command returns the current database structure version.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// Version outputs the current database structure version.
func Version(client *mongo.Client, dbName string, migrations []migration.Migration) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Getting the current database structure version.
    _, current, err := migrator.Run(dbName, "version")
    if err != nil {
        panic(err)
    }

    // Printing the current version.
    fmt.Printf("Current version: %d\n", current)
}
```

#### Version changing (set_version)

The `set_version` command changes the current version without applying migrations.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

// SetVersion changes the current database structure version number to the target.
func SetVersion(client *mongo.Client, dbName string, migrations []migration.Migration, target string) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Changing the database structure version.
    old, current, err := migrator.Run(dbName, "set_version", target)
    if err != nil {
        panic(err)
    }

    // Printing old and current versions.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

## Direct commands calls

In contrast, you can call commands functions directly. Be careful, direct calls receives `target` argument of type `int64` instead of `string` and `mongo.Database` pointer instead of database name.

```go
package usage_example

import (
    "fmt"

    "go.mongodb.org/mongo-driver/mongo"

    "github.com/Devoter/mongo-migrator/migrator"
    "github.com/Devoter/mongo-migrator/migration"
)

func DirectCalls(client *mongo.Client, dbName string, migrations []migration.Migration, target int64) {
    // Creating a migrator instance. The constructor sorts the migrations slice automatically
    // and appends the `zero` migration.
    migrator := migrator.NewMigrator(client, migrations)

    // Getting a database pointer.
    base := m.client.Database(dbName)

    // Creating the `migrations` collection and initialing it with the zero database structure version.
    if _, _, err := migrator.Init(base); err != nil {
        panic(err)
    }

    // Applying all unapplied migrations up to the target.
    old, current, err := migrator.Up(base, target)
    if err != nil {
        panic(err)
    }
    // Printing old and current versions.
    fmt.Printf("Old version: %d, current version: %d\n", old, current)

    // Applying all unapplied migrations.
    old, current, err = migrator.Up(base, -1)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Old version: %d, current version: %d\n", old, current)

    // Reverting the last applied migration.
    old, current, err = migrator.Down(base)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Old version: %d, current version: %d\n", old, current)

    // Reverting all applied migrations.
    old, current, err = migrator.Reset(base)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Old version: %d, current version: %d\n", old, current)

    // Changing the database structure version to target.
    old, current, err = migrator.SetVersion(base, target)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Old version: %d, current version: %d\n", old, current)

    // Getting the current database structure version.
    _, current, err = migrator.Version(base)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Current version: %d\n", current)

    // Changing the database structure version to zero.
    old, current, err = migrator.SetVersion(base, 0)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Old version: %d, current version: %d\n", old, current)
}
```

## License

[MIT](LICENSE)
