package migration

import (
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MajorityOpts returns `writeConcern: "majority"` collection option.
func MajorityOpts() *options.CollectionOptions {
	wMajority := writeconcern.New(writeconcern.WMajority())
	opts := options.Collection()
	opts.SetWriteConcern(wMajority)

	return opts
}
